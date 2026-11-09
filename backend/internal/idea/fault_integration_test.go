//go:build integration

package idea_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/dbtest"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/shared/web"
	"github.com/brian-huanggg/AwardRegistration/backend/internal/storage"
)

// Q4-01: fault tolerance of the edit paths (Sequence.md §3, §4, §7).

// Replace with a lost COMMIT acknowledgement. Unknown keeps the new object
// (recorded UPLOAD_ABORTED with the grace period) and answers 503; a known
// rollback deletes it now. The old object is never touched before a commit.
func TestReplaceSlidesLostCommit(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	e := newCreateEnvOn(t, d, faulty)
	leader := addUser(t, d, "100001", "USER")

	for i, tc := range []struct {
		name    string
		fault   dbtest.CommitFault
		applied bool // the pointer moved to the new object
		kept    bool // the new object stays, recorded with the grace period
	}{
		{"commit applied, ack lost", dbtest.LostAckCommitted, true, true},
		{"connection lost before commit", dbtest.LostAckNotCommitted, false, true},
		{"deadlock at commit", dbtest.DeadlockAtCommit, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "Replace " + string(rune('A'+i))
			id, oldKey := e.createdIdea(t, leader, name)
			before := len(e.objects.deleted)

			faults.FailNextCommit(tc.fault)
			rec := e.putSlides(t, leader, id, slidesName(name, "pdf"), pdf(20))
			if rec.Code != 503 || decodeProblem(t, rec).Code != "SERVICE_UNAVAILABLE" {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			newKey := e.objects.last
			var cur string
			if err := d.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, id.String()).Scan(&cur); err != nil {
				t.Fatal(err)
			}
			if (cur == newKey) != tc.applied {
				t.Errorf("pointer %q, new key %q, applied %v", cur, newKey, tc.applied)
			}
			_, newStored := e.objects.puts[newKey]
			got := e.pending(t, newKey)
			if tc.kept {
				grace := storage.UploadAbortedGrace.Seconds()
				if !newStored || len(got) != 1 || got[0].reason != "UPLOAD_ABORTED" ||
					got[0].dueInSec < grace-30 || got[0].dueInSec > grace+1 {
					t.Errorf("Unknown: stored %v, pending %+v; want kept and due after the grace", newStored, got)
				}
			} else if newStored || len(got) != 0 || len(e.objects.deleted) != before+1 {
				t.Errorf("rollback: stored %v, pending %+v; want deleted now", newStored, got)
			}
			// Whatever happened, the object the committed row points at exists,
			// and the old one is only released through the clean-up job.
			if _, ok := e.objects.puts[cur]; !ok {
				t.Errorf("row points at missing object %q", cur)
			}
			if _, ok := e.objects.puts[oldKey]; !ok {
				t.Error("old object deleted without a confirmed commit")
			}
		})
	}
	if faults.Fired() != 3 {
		t.Errorf("faults fired = %d, want 3", faults.Fired())
	}
}

// Delete with a lost COMMIT acknowledgement: 503 in every case, the object is
// never deleted inline, and the DELETED row exists only if the delete committed.
func TestDeleteIdeaLostCommit(t *testing.T) {
	dsn := dbtest.Start(t)
	d := dbtest.Open(t, dsn)
	faulty, faults := dbtest.OpenFaulty(t, dsn)
	e := newCreateEnvOn(t, d, faulty)
	leader := addUser(t, d, "100001", "USER")

	for i, tc := range []struct {
		name    string
		fault   dbtest.CommitFault
		applied bool // the idea row is gone
	}{
		{"commit applied, ack lost", dbtest.LostAckCommitted, true},
		{"connection lost before commit", dbtest.LostAckNotCommitted, false},
		{"deadlock at commit", dbtest.DeadlockAtCommit, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "Delete " + string(rune('A'+i))
			id, key := e.createdIdea(t, leader, name)

			faults.FailNextCommit(tc.fault)
			rec := e.del(leader, id)
			if rec.Code != 503 {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if n := e.count(t, `SELECT COUNT(*) FROM idea WHERE id = ?`, id.String()); (n == 0) != tc.applied {
				t.Errorf("idea rows = %d, applied %v", n, tc.applied)
			}
			if _, ok := e.objects.puts[key]; !ok {
				t.Error("object deleted although the commit is not confirmed")
			}
			wantPending := 0
			if tc.applied {
				wantPending = 1 // a later clean-up may remove it; the row must not be lost
			}
			if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion WHERE object_key = ?`, key); n != wantPending {
				t.Errorf("pending rows = %d, want %d", n, wantPending)
			}
		})
	}
	if faults.Fired() != 3 {
		t.Errorf("faults fired = %d, want 3", faults.Fired())
	}
}

// MinIO down before the upload: replace answers 503 STORAGE_UNAVAILABLE and
// changes nothing; the dedicated code lets the UI say "try again later".
func TestReplaceSlidesStorageDownIs503(t *testing.T) {
	e := newCreateEnv(t)
	leader := addUser(t, e.db, "100001", "USER")
	id, oldKey := e.createdIdea(t, leader, "Idea A")
	e.objects.fail = web.NewError(web.CodeStorageUnavailable, "object storage is unavailable")

	rec := e.putSlides(t, leader, id, slidesName("Idea A", "pdf"), pdf(20))
	if rec.Code != 503 || decodeProblem(t, rec).Code != "STORAGE_UNAVAILABLE" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var key string
	if err := e.db.QueryRow(`SELECT slides_object_key FROM idea WHERE id = ?`, id.String()).Scan(&key); err != nil || key != oldKey {
		t.Errorf("pointer moved: %q, %v", key, err)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion`); n != 0 {
		t.Errorf("pending rows %d, want 0", n)
	}
}

// S4 exit demo: the real MinIO container is stopped. Delete still succeeds and
// leaves the key pending; upload and download answer 503 STORAGE_UNAVAILABLE.
func TestMinIOStoppedDemo(t *testing.T) {
	ctx := context.Background()
	ctr, err := tcminio.Run(ctx, "cgr.dev/chainguard/minio:latest") // same image as docker-compose.yml
	if err != nil {
		t.Fatalf("start minio: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.WithoutCancel(ctx)) })
	ep, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg := storage.MinIOConfig{Endpoint: ep, AccessKey: ctr.Username, SecretKey: ctr.Password, Bucket: "slides"}
	admin, err := minio.New(ep, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	store, err := storage.NewMinIO(cfg)
	if err != nil {
		t.Fatal(err)
	}

	d := dbtest.New(t)
	e := newEnvWithStore(t, d, d, store)
	leader := addUser(t, d, "100001", "USER")
	create := func(name string) (uuid.UUID, string) {
		t.Helper()
		rec := e.post(t, leader, validForm(name))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %q: %d %s", name, rec.Code, rec.Body)
		}
		var sid, key string
		if err := d.QueryRow(`SELECT id, slides_object_key FROM idea WHERE name = ?`, name).Scan(&sid, &key); err != nil {
			t.Fatal(err)
		}
		return uuid.MustParse(sid), key
	}
	keep, _ := create("Keep")
	gone, goneKey := create("Gone")

	if err := ctr.Stop(ctx, nil); err != nil {
		t.Fatalf("stop minio: %v", err)
	}

	assert503 := func(what string, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != 503 || !strings.Contains(rec.Body.String(), "STORAGE_UNAVAILABLE") {
			t.Errorf("%s: %d %s, want 503 STORAGE_UNAVAILABLE", what, rec.Code, rec.Body)
		}
	}

	// Download.
	rec := e.download(t, leader, keep)
	assert503("download", rec)
	// Create.
	rec = e.post(t, leader, validForm("New"))
	assert503("create", rec)
	if n := e.count(t, `SELECT COUNT(*) FROM idea WHERE name = 'New'`); n != 0 {
		t.Errorf("failed create left %d rows", n)
	}
	// Replace.
	rec = e.putSlides(t, leader, keep, slidesName("Keep", "pdf"), pdf(20))
	assert503("replace", rec)
	// Delete succeeds; the key waits in pending_file_deletion.
	if rec := e.del(leader, gone); rec.Code != http.StatusNoContent {
		t.Fatalf("delete with MinIO down: %d %s", rec.Code, rec.Body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM idea WHERE id = ?`, gone.String()); n != 0 {
		t.Errorf("idea rows %d, want 0", n)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM pending_file_deletion WHERE object_key = ? AND reason = 'DELETED'`, goneKey); n != 1 {
		t.Errorf("DELETED rows %d, want 1", n)
	}
}
