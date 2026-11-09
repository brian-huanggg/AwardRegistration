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
import { can } from "@/shared/permissions";
import { Button } from "@/shared/ui/button";
import { PageHeading } from "../layouts";
import { NotFound } from "./NotFound";
import { useMe } from "../session";

export function IdeaList() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  return (
    <>
      <PageHeading title={t("nav.ideas")} />
      {me && (
        <IdeaListView
          admin={me.role === "ADMIN"}
          actions={
            can("createIdea", { me }).visible && (
              <Button asChild variant="outline">
                <Link to="/ideas/new">{t("pages.ideaList.create")}</Link>
              </Button>
            )
          }
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
