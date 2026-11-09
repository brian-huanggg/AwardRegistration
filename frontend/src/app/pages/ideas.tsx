import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate, useNavigate, useParams } from "react-router";
import { ApiError } from "@/shared/api";
import { deadlineKey, useDeadline } from "@/features/deadline/useDeadline";
import { fetchIdea, ideaKey } from "@/features/idea/detail";
import { DeleteIdea } from "@/features/idea/DeleteIdea";
import { ReplaceSlides } from "@/features/idea/ReplaceSlides";
import { IdeaForm } from "@/features/idea/IdeaForm";
import { IdeaDetail as IdeaDetailView } from "@/features/idea/IdeaDetail";
import { IdeaList as IdeaListView } from "@/features/idea/IdeaList";
import { IdeaRowActions } from "@/features/idea/IdeaRowActions";
import { Plus, UserPlus } from "lucide-react";
import { ApproveButton, ConfirmAction } from "@/features/team/MemberActions";
import { approveMember, leaveTeam, removeMember } from "@/features/team/members";
import { can } from "@/shared/permissions";
import { Button } from "@/shared/ui/button";
import { PageHeading } from "../layouts";
import { NotFound } from "./NotFound";
import { useMe } from "../session";

export function IdeaList() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  const client = useQueryClient();
  const admin = me?.role === "ADMIN";
  // Both header actions are User-only (UI-03); "Join a team" has no `can()` action of its own and follows "createIdea".
  const userActions = me ? can("createIdea", { me }).visible : false;
  return (
    <>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4">
        <div className="max-w-2xl space-y-1">
          <PageHeading title={t(admin ? "nav.allIdeas" : "nav.myIdeas")} />
          <p className="text-sm text-muted-foreground">
            {t(admin ? "pages.ideaList.descriptionAdmin" : "pages.ideaList.descriptionUser")}
          </p>
        </div>
        {userActions && (
          <div className="flex flex-wrap gap-2">
            <Button asChild variant="outline">
              <Link to="/join-requests">
                <UserPlus aria-hidden="true" />
                {t("pages.ideaList.joinTeam")}
              </Link>
            </Button>
            <Button asChild>
              <Link to="/ideas/new">
                <Plus aria-hidden="true" />
                {t("pages.ideaList.create")}
              </Link>
            </Button>
          </div>
        )}
      </div>
      {me && (
        <IdeaListView
          admin={admin}
          rowActions={(idea) => {
            // F6-01: Admin actions on the shared list; a User's summary has no members, so `can()` hides them.
            const download = can("downloadSlides", { me }).visible;
            const replace = can("replaceSlides", { me }).visible;
            const remove = can("deleteIdea", { me }).visible;
            return (
              <IdeaRowActions
                idea={idea}
                download={download}
                replace={replace}
                remove={remove}
                onDeleted={() => {
                  client.removeQueries({ queryKey: ideaKey(idea.id) });
                  void client.invalidateQueries({ queryKey: ["ideas"] });
                }}
              />
            );
          }}
        />
      )}
    </>
  );
}

export function IdeaCreate() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  const { data: deadline } = useDeadline();
  const client = useQueryClient();
  const navigate = useNavigate();
  const open = deadline?.state === "OPEN"; // Frontend.md §6.3 rule 6: a hint and disabled controls, not a guard
  return (
    <>
      <PageHeading title={t("pages.ideaCreate.title")} />
      <div className="max-w-2xl rounded-xl border bg-card p-6">
        {me && (
          <IdeaForm
            leader={{ name: me.name, empNo: me.empNo }}
            canSubmit={open}
            onOutcomeUnknown={() =>
              void client.invalidateQueries({ queryKey: ["ideas"] })
            }
            onSubmissionRejected={() =>
              void client.invalidateQueries({ queryKey: deadlineKey })
            }
            onSaved={(idea) => {
              // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
              client.setQueryData(["idea", idea.id], idea);
              void client.invalidateQueries({ queryKey: ["ideas"] });
              void navigate(`/ideas/${idea.id}`);
            }}
          />
        )}
      </div>
    </>
  );
}

export function IdeaDetail() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const { data: me } = useMe();
  const { data: deadline } = useDeadline();
  const client = useQueryClient();
  const navigate = useNavigate();
  return (
    <>
      {me && (
        <IdeaDetailView
          id={id}
          admin={me.role === "ADMIN"}
          notFound={<NotFound />}
          editAction={(idea) =>
            can("editIdea", { me, idea }).visible ? (
              <Button asChild variant="outline">
                <Link to={`/ideas/${idea.id}/edit`}>
                  {t("pages.ideaEdit.edit")}
                </Link>
              </Button>
            ) : null
          }
          slideActions={(idea) => {
            // Frontend.md §5.4: a hint only; the Leader is limited to the submission window, an Admin is not.
            const ctx = { me, idea, state: deadline?.state };
            const replace = can("replaceSlides", ctx);
            const del = can("deleteIdea", ctx);
            const rejected = () =>
              void client.invalidateQueries({ queryKey: deadlineKey });
            return (
              <>
                {replace.visible && (
                  <ReplaceSlides
                    idea={idea}
                    disabled={!replace.enabled}
                    onSubmissionRejected={rejected}
                    onOutcomeUnknown={() =>
                      void client.invalidateQueries({
                        queryKey: ideaKey(idea.id),
                      })
                    }
                    onReplaced={(saved) => {
                      // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
                      client.setQueryData(ideaKey(saved.id), saved);
                      void client.invalidateQueries({ queryKey: ["ideas"] });
                    }}
                  />
                )}
                {del.visible && (
                  <DeleteIdea
                    idea={idea}
                    disabled={!del.enabled}
                    onSubmissionRejected={rejected}
                    onDeleted={() => {
                      client.removeQueries({ queryKey: ideaKey(idea.id) });
                      void client.invalidateQueries({ queryKey: ["ideas"] });
                      void navigate("/ideas", { replace: true });
                    }}
                  />
                )}
              </>
            );
          }}
          memberActions={(idea, m) => {
            const ctx = { me, idea, state: deadline?.state };
            const own = m.empNo === me.empNo;
            const gone = () => void client.invalidateQueries({ queryKey: ideaKey(idea.id) });
            const stale = (e: ApiError) => {
              gone();
              if (e.code.startsWith("SUBMISSION_")) void client.invalidateQueries({ queryKey: deadlineKey });
            };
            const label = `${m.name} (${m.empNo})`;
            if (own) {
              const leave = can("leaveTeam", ctx);
              if (!leave.visible) return null;
              return (
                <ConfirmAction
                  trigger={t("pages.ideaDetail.leave")}
                  triggerLabel={t("pages.ideaDetail.leave")}
                  title={t("pages.ideaDetail.leaveTitle")}
                  body={t("pages.ideaDetail.leaveBody", { name: idea.name })}
                  confirm={t("pages.ideaDetail.leaveConfirm")}
                  busy={t("pages.ideaDetail.leaving")}
                  disabled={!leave.enabled}
                  run={async () => {
                    await leaveTeam(idea.id);
                    // the Idea is no longer visible to the caller: forget it and go to the list
                    client.removeQueries({ queryKey: ideaKey(idea.id) });
                    void client.invalidateQueries({ queryKey: ["ideas"] });
                    void navigate("/ideas", { replace: true });
                  }}
                  onGone={() => {
                    client.removeQueries({ queryKey: ideaKey(idea.id) });
                    void client.invalidateQueries({ queryKey: ["ideas"] });
                    void navigate("/ideas", { replace: true });
                  }}
                  onStale={stale}
                />
              );
            }
            const remove = can("removeMember", ctx);
            if (!remove.visible) return null;
            return (
              <ConfirmAction
                trigger={t("pages.ideaDetail.remove")}
                triggerLabel={t("pages.ideaDetail.removeNamed", { name: label })}
                title={t("pages.ideaDetail.removeTitle")}
                body={t("pages.ideaDetail.removeBody", { name: label })}
                confirm={t("pages.ideaDetail.removeConfirm")}
                busy={t("pages.ideaDetail.removing")}
                disabled={!remove.enabled}
                run={async () => {
                  await removeMember(idea.id, m.empNo);
                  gone();
                }}
                onGone={gone}
                onStale={stale}
              />
            );
          }}
          pendingActions={(idea, r) => {
            const ctx = { me, idea, state: deadline?.state };
            const approve = can("approveMember", ctx);
            const reject = can("removeMember", ctx);
            const gone = () => void client.invalidateQueries({ queryKey: ideaKey(idea.id) });
            const stale = (e: ApiError) => {
              gone();
              if (e.code.startsWith("SUBMISSION_")) void client.invalidateQueries({ queryKey: deadlineKey });
            };
            const label = `${r.name} (${r.empNo})`;
            return (
              <>
                {approve.visible && (
                  <ApproveButton
                    name={label}
                    disabled={!approve.enabled}
                    run={async () => {
                      const saved = await approveMember(idea.id, r.empNo);
                      // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
                      client.setQueryData(ideaKey(saved.id), saved);
                      void client.invalidateQueries({ queryKey: ["ideas"] });
                    }}
                    onGone={gone}
                    onStale={stale}
                  />
                )}
                {reject.visible && (
                  <ConfirmAction
                    trigger={t("pages.ideaDetail.reject")}
                    triggerLabel={t("pages.ideaDetail.rejectNamed", { name: label })}
                    title={t("pages.ideaDetail.rejectTitle")}
                    body={t("pages.ideaDetail.rejectBody", { name: label })}
                    confirm={t("pages.ideaDetail.rejectConfirm")}
                    busy={t("pages.ideaDetail.rejecting")}
                    disabled={!reject.enabled}
                    run={async () => {
                      await removeMember(idea.id, r.empNo);
                      gone();
                    }}
                    onGone={gone}
                    onStale={stale}
                  />
                )}
              </>
            );
          }}
          heading={<PageHeading title={t("pages.ideaDetail.title")} />}
        />
      )}
      <Link
        to="/ideas"
        className="font-medium text-primary underline underline-offset-4"
      >
        {t("pages.ideaDetail.back")}
      </Link>
    </>
  );
}

export function IdeaEdit() {
  const { t, i18n } = useTranslation();
  const { id = "" } = useParams();
  const { data: me } = useMe();
  const { data: deadline } = useDeadline();
  const client = useQueryClient();
  const navigate = useNavigate();
  const {
    data: idea,
    isPending,
    error,
    refetch,
  } = useQuery({
    queryKey: ideaKey(id),
    queryFn: ({ signal }) => fetchIdea(id, signal),
    retry: false,
    staleTime: 30_000,
  });
  const heading = <PageHeading title={t("pages.ideaEdit.title")} />;
  if (error instanceof ApiError && error.status === 404) return <NotFound />;
  if (isPending && !error)
    return (
      <>
        {heading}
        <p role="status">{t("common.loading")}</p>
      </>
    );
  if (!idea || !me) {
    const key =
      error instanceof ApiError && i18n.exists(`errors.${error.code}`)
        ? error.code
        : "UNKNOWN";
    return (
      <>
        {heading}
        <div role="alert">
          <p className="mb-2">{t(`errors.${key}`)}</p>
          <Button variant="outline" onClick={() => void refetch()}>
            {t("common.retry")}
          </Button>
        </div>
      </>
    );
  }
  // A Member who is not the Leader may see the Idea but not edit it (Frontend.md §6.2).
  if (idea.leader.empNo !== me.empNo)
    return <Navigate to={`/ideas/${id}`} replace />;
  const back = `/ideas/${id}`;
  return (
    <>
      {heading}
      <div className="max-w-2xl rounded-xl border bg-card p-6">
        <p className="mb-4 text-sm text-muted-foreground">
          {t("pages.ideaEdit.slidesNote")}
        </p>
        <IdeaForm
          key={`${idea.id}-${idea.version}`}
          leader={{ name: idea.leader.name, empNo: idea.leader.empNo }}
          edit={idea}
          canSubmit={deadline?.state === "OPEN"}
          onSubmissionRejected={() =>
            void client.invalidateQueries({ queryKey: deadlineKey })
          }
          onCancel={
            <Button asChild variant="outline">
              <Link to={back}>{t("common.cancel")}</Link>
            </Button>
          }
          onSaved={(saved) => {
            // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
            client.setQueryData(ideaKey(saved.id), saved);
            void client.invalidateQueries({ queryKey: ["ideas"] });
            void navigate(back);
          }}
        />
      </div>
    </>
  );
}
