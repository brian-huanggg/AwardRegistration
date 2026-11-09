import { useTranslation } from "react-i18next";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, Navigate, useLocation, useNavigate, useParams } from "react-router";
import { ArrowLeft, Check, LogOut, Pencil, UserMinus, X } from "lucide-react";
import { ApiError } from "@/shared/api";
import { deadlineKey, useDeadline } from "@/features/deadline/useDeadline";
import { DeadlineNotice } from "@/features/deadline/DeadlineNotice";
import { useServerNow } from "@/features/deadline/useServerNow";
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
import { JoinRequestsCard, TeamCard, TEAM_SIZE } from "@/features/team/TeamCards";
import { can } from "@/shared/permissions";
import { Button } from "@/shared/ui/button";
import { ideasKey } from "../breadcrumb";
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
      <div className="max-w-[55rem]">
        {me && (
          <IdeaForm
            leader={{ name: me.name, empNo: me.empNo }}
            canSubmit={open}
            onCancel={
              <Button asChild variant="outline">
                <Link to="/ideas">{t("common.cancel")}</Link>
              </Button>
            }
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
  const serverNow = useServerNow();
  const client = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  // UI-04: back to the list with its filters when the list put its URL in history state
  const from = (location.state as { from?: unknown } | null)?.from;
  const listUrl = typeof from === "string" && from.startsWith("/ideas") ? from : "/ideas";
  const toList = () => {
    void client.invalidateQueries({ queryKey: ["ideas"] });
    void navigate("/ideas", { replace: true });
  };
  if (!me) return null;
  const back = (
    <Link
      to={listUrl}
      className="inline-flex items-center gap-1.5 text-sm font-medium text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
    >
      <ArrowLeft aria-hidden="true" className="size-4" />
      {t("pages.ideaDetail.backTo", { list: t(ideasKey(me.role)) })}
    </Link>
  );
  return (
    <IdeaDetailView
      id={id}
      me={me}
      notFound={<NotFound />}
      back={back}
      heading={(title) => (
        <PageHeading title={title ?? t("pages.ideaDetail.title")} />
      )}
      notice={<DeadlineNotice />}
      headerActions={(idea) => {
        // Frontend.md §5.4: a hint only; the Leader is limited to the submission window, an Admin is not.
        const ctx = { me, idea, state: deadline?.state };
        const edit = can("editIdea", ctx);
        const del = can("deleteIdea", ctx);
        const leave = can("leaveTeam", ctx);
        return (
          <>
            {edit.visible && (
              <Button asChild variant="outline" size="sm">
                <Link to={`/ideas/${idea.id}/edit`}>
                  <Pencil aria-hidden="true" />
                  {t("pages.ideaDetail.edit")}
                </Link>
              </Button>
            )}
            {del.visible && (
              <DeleteIdea
                idea={idea}
                disabled={!del.enabled}
                onSubmissionRejected={() =>
                  void client.invalidateQueries({ queryKey: deadlineKey })
                }
                onDeleted={() => {
                  client.removeQueries({ queryKey: ideaKey(idea.id) });
                  toList();
                }}
              />
            )}
            {leave.visible && (
              <ConfirmAction
                trigger={t("pages.ideaDetail.leave")}
                triggerLabel={t("pages.ideaDetail.leave")}
                icon={<LogOut aria-hidden="true" />}
                title={t("pages.ideaDetail.leaveTitle")}
                body={t("pages.ideaDetail.leaveBody", { name: idea.name })}
                confirm={t("pages.ideaDetail.leaveConfirm")}
                busy={t("pages.ideaDetail.leaving")}
                disabled={!leave.enabled}
                run={async () => {
                  await leaveTeam(idea.id);
                  // the Idea is no longer visible to the caller: forget it and go to the list
                  client.removeQueries({ queryKey: ideaKey(idea.id) });
                  toList();
                }}
                onGone={() => {
                  client.removeQueries({ queryKey: ideaKey(idea.id) });
                  toList();
                }}
                onStale={(e) => {
                  void client.invalidateQueries({ queryKey: ideaKey(idea.id) });
                  if (e.code.startsWith("SUBMISSION_")) void client.invalidateQueries({ queryKey: deadlineKey });
                }}
              />
            )}
          </>
        );
      }}
      slideActions={(idea) => {
        const replace = can("replaceSlides", { me, idea, state: deadline?.state });
        if (!replace.visible) return null;
        return (
          <ReplaceSlides
            idea={idea}
            disabled={!replace.enabled}
            onSubmissionRejected={() =>
              void client.invalidateQueries({ queryKey: deadlineKey })
            }
            onOutcomeUnknown={() =>
              void client.invalidateQueries({ queryKey: ideaKey(idea.id) })
            }
            onReplaced={(saved) => {
              // the response is the detail: cache it, no follow-up GET (Frontend.md §6.7)
              client.setQueryData(ideaKey(saved.id), saved);
              void client.invalidateQueries({ queryKey: ["ideas"] });
            }}
          />
        );
      }}
      side={(idea) => {
        const ctx = { me, idea, state: deadline?.state };
        const leader = idea.leader.empNo === me.empNo;
        const remove = can("removeMember", ctx); // also covers rejecting a pending request
        const approve = can("approveMember", ctx);
        const full = 1 + idea.members.length >= TEAM_SIZE;
        const gone = () => void client.invalidateQueries({ queryKey: ideaKey(idea.id) });
        const stale = (e: ApiError) => {
          gone();
          if (e.code.startsWith("SUBMISSION_")) void client.invalidateQueries({ queryKey: deadlineKey });
        };
        return (
          <>
            <TeamCard
              leader={idea.leader}
              members={idea.members}
              meEmpNo={me.empNo}
              ideaName={idea.name}
              showOpenSeats={leader}
              memberActions={(m) => {
                if (!remove.visible || m.empNo === me.empNo) return null;
                const label = `${m.name} (${m.empNo})`;
                return (
                  <ConfirmAction
                    trigger={t("pages.ideaDetail.remove")}
                    triggerLabel={t("pages.ideaDetail.removeNamed", { name: label })}
                    icon={<UserMinus aria-hidden="true" />}
                    iconOnly
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
            />
            {idea.pendingRequests !== null && (
              <JoinRequestsCard
                requests={idea.pendingRequests}
                hint={t(leader ? "pages.ideaDetail.requestsHintLeader" : "pages.ideaDetail.requestsHintAdmin")}
                footer={approve.visible ? t("pages.ideaDetail.requestsFooter") : undefined}
                now={serverNow}
                actions={(r) => {
                  const label = `${r.name} (${r.empNo})`;
                  return (
                    <>
                      {remove.visible && (
                        <ConfirmAction
                          trigger={t("pages.ideaDetail.reject")}
                          triggerLabel={t("pages.ideaDetail.rejectNamed", { name: label })}
                          icon={<X aria-hidden="true" />}
                          title={t("pages.ideaDetail.rejectTitle")}
                          body={t("pages.ideaDetail.rejectBody", { name: label })}
                          confirm={t("pages.ideaDetail.rejectConfirm")}
                          busy={t("pages.ideaDetail.rejecting")}
                          disabled={!remove.enabled}
                          run={async () => {
                            await removeMember(idea.id, r.empNo);
                            gone();
                          }}
                          onGone={gone}
                          onStale={stale}
                        />
                      )}
                      {/* hidden at 4/4; the server's 409 TEAM_FULL stays the judge */}
                      {approve.visible && !full && (
                        <ApproveButton
                          name={label}
                          icon={<Check aria-hidden="true" />}
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
                    </>
                  );
                }}
              />
            )}
          </>
        );
      }}
    />
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
      <div className="max-w-[55rem]">
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
