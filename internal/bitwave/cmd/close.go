package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bitwave-io/bitwave-cli/internal/orgreports"
)

// close exposes the Bitwave Close Dashboard's backend surface. It deliberately
// does not decide which tasks to run, approve exceptions, or choose accounting
// treatment: an operator or agent such as Wavie owns that orchestration.
func newCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Inspect and operate Bitwave Close Dashboard workflows",
		Long: `Expose the same checklist, task, certification, delivery, and supporting
endpoints used by Bitwave's Close Dashboard.

A checklist run represents one close period. Its tasks hold the evidence,
alerts, dependencies, and outputs shown by the dashboard. The CLI returns that
server state as structured JSON; it does not decide what should pass, which
exceptions should be accepted, or which accounting treatment is appropriate.

All writes require --yes and support --dry-run. Wavie or another caller should
inspect run and task state between mutations rather than assuming success.`,
	}
	cmd.AddCommand(
		newCloseListCmd(), newCloseGetCmd(), newCloseCreateCmd(), newCloseUpdateCmd(), newCloseRunCmd(),
		newCloseCompleteCmd(), newCloseDeleteCmd(), newCloseHardCmd(),
		newCloseTaskCmd(), newCloseInvocationCmd(), newCloseTemplateCmd(),
		newCloseCertificationCmd(), newCloseArtifactCmd(), newCloseDeliveryCmd(),
		newCloseInventoryActionsCmd(), newCloseExportCmd(), newCloseRollupConfigCmd(),
	)
	return cmd
}

func newCloseListCmd() *cobra.Command {
	var orgID, templateID, from, to string
	cmd := &cobra.Command{Use: "list", Short: "List close checklist runs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		query := url.Values{}
		addQuery(query, "templateId", templateID)
		addQuery(query, "startDate", from)
		addQuery(query, "endDate", to)
		return closeRead(cmd, orgID, "/v3/orgs/{org}/checklists", query)
	}}
	addCloseReadFlags(cmd, &orgID)
	cmd.Flags().StringVar(&templateID, "template", "", "Checklist template ID")
	cmd.Flags().StringVar(&from, "from", "", "Period start-date filter")
	cmd.Flags().StringVar(&to, "to", "", "Period end-date filter")
	return cmd
}

func newCloseGetCmd() *cobra.Command {
	var orgID string
	cmd := &cobra.Command{Use: "get RUN_ID", Short: "Get a close run, including progress and task state", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return closeRead(cmd, orgID, closeRunPath(args[0]), nil)
	}}
	addCloseReadFlags(cmd, &orgID)
	return cmd
}

func newCloseCreateCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "create", short: "Create a version-2 close checklist run", operation: "create-close-run",
		method: http.MethodPost, path: "/v3/orgs/{org}/checklists", inputRequired: true,
		inputHelp: "JSON request with parameters and version (normally 2)",
		long:      "Creates a checklist run. The request normally has {\"parameters\":{\"templateId\":...,\"startDate\":...,\"endDate\":...},\"version\":2}.",
	})
}

func newCloseUpdateCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "update RUN_ID", short: "Update close-run parameters or its stored template", operation: "update-close-run",
		method: http.MethodPatch, pathFromArg: func(args []string) string { return closeRunPath(args[0]) },
		inputRequired: true, eTag: true, inputHelp: "Complete close-run patch JSON",
		long: "Exposes the dashboard's run update endpoint. Use the eTag from the latest run read; this does not execute tasks.",
	})
}

func newCloseRunCmd() *cobra.Command {
	var f closeMutationFlags
	var taskIDs []string
	var includeFailed bool
	cmd := &cobra.Command{
		Use: "run RUN_ID", Short: "Start runnable tasks in a close checklist", Args: cobra.ExactArgs(1),
		Long: "Starts every runnable task, or only --task IDs. This returns immediately; inspect `close get` or individual tasks/invocations for server state.",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if len(taskIDs) > 0 {
				body["taskIds"] = uniqueNonEmpty(taskIDs)
			}
			if cmd.Flags().Changed("include-failed") {
				body["includeFailed"] = includeFailed
			}
			return closeMutation(cmd, f, "run-close-checklist", http.MethodPost, closeRunPath(args[0])+"/run", body, nil)
		},
	}
	addCloseMutationFlags(cmd, &f)
	cmd.Flags().StringSliceVar(&taskIDs, "task", nil, "Only start these task IDs (repeatable or comma-separated)")
	cmd.Flags().BoolVar(&includeFailed, "include-failed", false, "Include failed tasks when starting the run")
	return cmd
}

func newCloseCompleteCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "complete RUN_ID", short: "Mark a close checklist run complete", operation: "complete-close-run",
		method: http.MethodPatch, pathFromArg: func(args []string) string { return closeRunPath(args[0]) + "/complete" },
		inputRequired: true, eTag: true, inputHelp: "Completion JSON (notes, override, closedBy, and any server-required fields)",
		long: "Completes the run after its tasks have reached appropriate terminal states. --etag is sent as If-Match for optimistic concurrency.",
	})
}

func newCloseDeleteCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "delete RUN_ID", short: "Delete a close checklist run", operation: "delete-close-run",
		method: http.MethodDelete, pathFromArg: func(args []string) string { return closeRunPath(args[0]) },
	})
}

func newCloseHardCmd() *cobra.Command {
	var f closeMutationFlags
	var endDate string
	cmd := &cobra.Command{Use: "hard-close", Short: "Set the organization's hard-close date", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(endDate) == "" {
			return errors.New("--end-date is required")
		}
		query := `mutation updateOrg($orgId: ID!, $hardCloseDate: String!) { updateOrg(orgId: $orgId, hardCloseDate: $hardCloseDate) { id } }`
		orgID, err := resolveReportOrg(f.orgID)
		if err != nil {
			return err
		}
		payload := map[string]any{"query": query, "variables": map[string]any{"orgId": orgID, "hardCloseDate": endDate}}
		return closeMutationResolved(cmd, f, orgID, "set-hard-close-date", orgreports.APIServiceApp, http.MethodPost, "/graphql", payload, nil)
	}}
	addCloseMutationFlags(cmd, &f)
	cmd.Flags().StringVar(&endDate, "end-date", "", "Hard-close date (normally YYYY-MM-DD)")
	return cmd
}

func newCloseTaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Aliases: []string{"tasks"}, Short: "Inspect, run, or review close tasks"}
	cmd.AddCommand(newCloseTaskListCmd(), newCloseTaskGetCmd(), newCloseTaskRunCmd(), newCloseTaskUpdateCmd(), newCloseTaskAcceptCmd(), newCloseTaskUnacceptCmd())
	return cmd
}

func newCloseTaskListCmd() *cobra.Command {
	var orgID string
	cmd := &cobra.Command{Use: "list RUN_ID", Short: "List tasks in a close run", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return closeRead(cmd, orgID, closeRunPath(args[0])+"/tasks", nil)
	}}
	addCloseReadFlags(cmd, &orgID)
	return cmd
}

func newCloseTaskGetCmd() *cobra.Command {
	var orgID string
	cmd := &cobra.Command{Use: "get RUN_ID TASK_ID", Short: "Get one close task and its evidence, alerts, and output", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return closeRead(cmd, orgID, closeTaskPath(args[0], args[1]), nil)
	}}
	addCloseReadFlags(cmd, &orgID)
	return cmd
}

func newCloseTaskRunCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "run RUN_ID TASK_ID", short: "Start or retry one close task", operation: "run-close-task",
		method: http.MethodPost, pathFromArg: func(args []string) string { return closeTaskPath(args[0], args[1]) + "/run" },
		inputHelp: "Optional task overrides such as connectionId or destinationPath",
		long:      "Starts one task and returns immediately. An empty body is valid; use --input only for task-specific overrides.",
	})
}

func newCloseTaskUpdateCmd() *cobra.Command {
	return newCloseJSONMutationCmd(closeMutationSpec{
		use: "update RUN_ID TASK_ID", short: "Update a close task using the backend patch contract", operation: "update-close-task",
		method: http.MethodPatch, pathFromArg: func(args []string) string { return closeTaskPath(args[0], args[1]) },
		inputRequired: true, eTag: true, inputHelp: "Complete task patch JSON",
	})
}

func newCloseTaskAcceptCmd() *cobra.Command {
	var f closeMutationFlags
	var reason string
	cmd := &cobra.Command{Use: "accept RUN_ID TASK_ID", Short: "Accept a task that requires human review", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		accept := map[string]any{}
		if reason != "" {
			accept["reason"] = reason
		}
		return closeMutation(cmd, f, "accept-close-task", http.MethodPatch, closeTaskPath(args[0], args[1]), map[string]any{"accept": accept}, nil)
	}}
	addCloseMutationFlags(cmd, &f)
	cmd.Flags().StringVar(&reason, "reason", "", "Acceptance reason recorded on the task")
	return cmd
}

func newCloseTaskUnacceptCmd() *cobra.Command {
	var f closeMutationFlags
	cmd := &cobra.Command{Use: "unaccept RUN_ID TASK_ID", Short: "Reverse a prior close-task acceptance", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		return closeMutation(cmd, f, "unaccept-close-task", http.MethodPatch, closeTaskPath(args[0], args[1]), map[string]any{"unaccept": true}, nil)
	}}
	addCloseMutationFlags(cmd, &f)
	return cmd
}

func newCloseInvocationCmd() *cobra.Command {
	var orgID string
	cmd := &cobra.Command{Use: "invocation", Short: "Inspect asynchronous close-task invocations"}
	get := &cobra.Command{Use: "get INVOCATION_ID", Short: "Get invocation status and output", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return closeRead(cmd, orgID, "/v3/orgs/{org}/invocations/"+url.PathEscape(args[0]), nil)
	}}
	addCloseReadFlags(get, &orgID)
	cmd.AddCommand(get)
	return cmd
}

func newCloseTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "template", Aliases: []string{"templates"}, Short: "Inspect or manage close checklist templates"}
	var listOrg, catalogOrg, getOrg string
	list := &cobra.Command{Use: "list", Short: "List available close templates", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return closeRead(c, listOrg, closeTemplatePath(""), nil) }}
	catalog := &cobra.Command{Use: "catalog", Short: "List task types available to custom templates", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		return closeRead(c, catalogOrg, closeTemplatePath("catalog"), nil)
	}}
	get := &cobra.Command{Use: "get TEMPLATE_ID", Short: "Get a close template definition", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return closeRead(c, getOrg, closeTemplatePath(args[0]), nil)
	}}
	addCloseReadFlags(list, &listOrg)
	addCloseReadFlags(catalog, &catalogOrg)
	addCloseReadFlags(get, &getOrg)
	cmd.AddCommand(list, catalog, get)
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "create", short: "Create a custom close template", operation: "create-close-template", method: http.MethodPost, path: closeTemplatePath(""), inputRequired: true, inputHelp: "JSON with templateName and template"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "update TEMPLATE_ID", short: "Update a custom close template", operation: "update-close-template", method: http.MethodPatch, pathFromArg: func(a []string) string { return closeTemplatePath(a[0]) }, inputRequired: true, eTag: true, inputHelp: "JSON with template and optional templateName"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "delete TEMPLATE_ID", short: "Delete a custom close template", operation: "delete-close-template", method: http.MethodDelete, pathFromArg: func(a []string) string { return closeTemplatePath(a[0]) }, eTag: true}))
	return cmd
}

func newCloseCertificationCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "certification", Aliases: []string{"certifications"}, Short: "Generate and retrieve close certifications"}
	var listOrg, getOrg, statusOrg, downloadOrg string
	var limit int
	var cursor string
	list := &cobra.Command{Use: "list", Short: "List generated close certifications", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		q := url.Values{}
		if limit > 0 {
			q.Set("limit", fmt.Sprint(limit))
		}
		addQuery(q, "cursor", cursor)
		return closeRead(c, listOrg, "/v2/orgs/{org}/close-reports", q)
	}}
	get := &cobra.Command{Use: "get REPORT_ID", Short: "Get close certification details and check results", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, getOrg, "/v2/orgs/{org}/close-reports/"+url.PathEscape(a[0]), nil)
	}}
	status := &cobra.Command{Use: "status REPORT_RUN_ID", Short: "Get close-certification workflow status", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, statusOrg, "/v2/orgs/{org}/close-reports/workflow-status/"+url.PathEscape(a[0]), nil)
	}}
	download := &cobra.Command{Use: "download-link REPORT_ID", Short: "Get the signed PDF download URL", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, downloadOrg, "/v2/orgs/{org}/close-reports/"+url.PathEscape(a[0])+"/download", nil)
	}}
	addCloseReadFlags(list, &listOrg)
	addCloseReadFlags(get, &getOrg)
	addCloseReadFlags(status, &statusOrg)
	addCloseReadFlags(download, &downloadOrg)
	list.Flags().IntVar(&limit, "limit", 100, "Maximum certifications per page")
	list.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.AddCommand(list, get, status, download)
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "create", short: "Generate a close certification for a checklist run", operation: "generate-close-certification", method: http.MethodPost, path: "/v2/orgs/{org}/close-reports", inputRequired: true, inputHelp: "JSON containing checkListRunId"}))
	return cmd
}

func newCloseArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "artifact", Aliases: []string{"report-artifact"}, Short: "Inspect reports produced by close tasks"}
	var getOrg, linkOrg, snapshotOrg string
	get := &cobra.Command{Use: "get REPORT_RUN_ID", Short: "Get a generated report record", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, getOrg, "/v2/orgs/{org}/reports/"+url.PathEscape(a[0]), nil)
	}}
	link := &cobra.Command{Use: "download-link REPORT_RUN_ID", Short: "Get download links for a generated report", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		q := url.Values{"includeDownloadUrls": {"true"}}
		return closeRead(c, linkOrg, "/v2/orgs/{org}/reports/"+url.PathEscape(a[0]), q)
	}}
	snapshot := &cobra.Command{Use: "snapshot REPORT_ID", Short: "Read a close report snapshot from Bitwave storage", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, snapshotOrg, "/v2/orgs/{org}/reports/snapshots/sftp_files/file/"+url.PathEscape(a[0]), nil)
	}}
	addCloseReadFlags(get, &getOrg)
	addCloseReadFlags(link, &linkOrg)
	addCloseReadFlags(snapshot, &snapshotOrg)
	cmd.AddCommand(get, link, snapshot)
	return cmd
}

func newCloseDeliveryCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "delivery", Short: "Manage SFTP destinations and per-run report assignments"}
	cmd.AddCommand(newCloseSFTPPathCmd(), newCloseSFTPAssignmentCmd())
	return cmd
}

func newCloseSFTPPathCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "path", Short: "Manage reusable SFTP paths"}
	var listOrg, getOrg, connectionID, action string
	list := &cobra.Command{Use: "list", Short: "List configured SFTP paths", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		q := url.Values{}
		addQuery(q, "connectionId", connectionID)
		addQuery(q, "action", action)
		return closeRead(c, listOrg, "/orgs/{org}/sftp-paths", q)
	}}
	get := &cobra.Command{Use: "get PATH_ID", Short: "Get one SFTP path", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, getOrg, "/orgs/{org}/sftp-paths/"+url.PathEscape(a[0]), nil)
	}}
	addCloseReadFlags(list, &listOrg)
	addCloseReadFlags(get, &getOrg)
	list.Flags().StringVar(&connectionID, "connection", "", "SFTP connection ID")
	list.Flags().StringVar(&action, "action", "", "Path action, such as Push")
	cmd.AddCommand(list, get)
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "test", short: "Test SFTP path reachability and permissions", operation: "test-sftp-path", method: http.MethodPost, path: "/orgs/{org}/sftp-paths/test", inputRequired: true, inputHelp: "SFTP path JSON"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "create", short: "Create an SFTP path", operation: "create-sftp-path", method: http.MethodPost, path: "/orgs/{org}/sftp-paths", inputRequired: true, inputHelp: "SFTP path JSON"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "update PATH_ID", short: "Update an SFTP path", operation: "update-sftp-path", method: http.MethodPatch, pathFromArg: func(a []string) string { return "/orgs/{org}/sftp-paths/" + url.PathEscape(a[0]) }, inputRequired: true, inputHelp: "SFTP path patch JSON"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "delete PATH_ID", short: "Delete an SFTP path", operation: "delete-sftp-path", method: http.MethodDelete, pathFromArg: func(a []string) string { return "/orgs/{org}/sftp-paths/" + url.PathEscape(a[0]) }}))
	return cmd
}

func newCloseSFTPAssignmentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "assignment", Short: "Assign close reports to SFTP paths"}
	var listOrg string
	list := &cobra.Command{Use: "list RUN_ID", Short: "List report-path assignments for a close run", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error { return closeRead(c, listOrg, closeAssignmentBase(a[0]), nil) }}
	addCloseReadFlags(list, &listOrg)
	cmd.AddCommand(list)
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "set RUN_ID REPORT_ID", short: "Set a report's SFTP path", operation: "set-close-sftp-assignment", method: http.MethodPut, pathFromArg: func(a []string) string {
		return "/orgs/{org}/checklist-runs/" + url.PathEscape(a[0]) + "/reports/" + url.PathEscape(a[1]) + "/sftp-path"
	}, inputRequired: true, inputHelp: "JSON containing sftpPathId"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "bulk-set RUN_ID", short: "Set multiple report-path assignments", operation: "bulk-set-close-sftp-assignments", method: http.MethodPost, pathFromArg: func(a []string) string { return closeAssignmentBase(a[0]) }, inputRequired: true, inputHelp: "JSON containing assignments[]"}))
	cmd.AddCommand(newCloseJSONMutationCmd(closeMutationSpec{use: "delete RUN_ID REPORT_ID", short: "Delete a report-path assignment", operation: "delete-close-sftp-assignment", method: http.MethodDelete, pathFromArg: func(a []string) string {
		return "/orgs/{org}/checklist-runs/" + url.PathEscape(a[0]) + "/reports/" + url.PathEscape(a[1]) + "/sftp-path"
	}}))
	return cmd
}

func newCloseInventoryActionsCmd() *cobra.Command {
	var orgID, asOf string
	var export bool
	cmd := &cobra.Command{Use: "inventory-actions VIEW_ID", Short: "Read the inventory actions used by close checks", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		q := url.Values{}
		addQuery(q, "asOf", asOf)
		q.Set("exportResults", fmt.Sprint(export))
		return closeRead(c, orgID, "/orgs/{org}/inventory-views/"+url.PathEscape(a[0])+"/actions", q)
	}}
	addCloseReadFlags(cmd, &orgID)
	cmd.Flags().StringVar(&asOf, "as-of", "", "Actions end date")
	cmd.Flags().BoolVar(&export, "export-results", true, "Create export results as the dashboard does")
	return cmd
}

func newCloseExportCmd() *cobra.Command {
	var orgID string
	var rawURL bool
	cmd := &cobra.Command{Use: "export EXPORT_ID", Short: "Get a close-supporting export result", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		q := url.Values{}
		q.Set("rawUrl", fmt.Sprint(rawURL))
		return closeRead(c, orgID, "/v2/orgs/{org}/exports/"+url.PathEscape(a[0]), q)
	}}
	addCloseReadFlags(cmd, &orgID)
	cmd.Flags().BoolVar(&rawURL, "raw-url", true, "Return the raw signed result URL")
	return cmd
}

func newCloseRollupConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "rollup-config", Short: "Inspect rolled-up JE configurations selectable by close templates"}
	var listOrg, getOrg string
	var activeOnly, includeDeleted bool
	list := &cobra.Command{Use: "list", Short: "List rolled-up JE configurations", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		q := url.Values{}
		q.Set("activeOnly", fmt.Sprint(activeOnly))
		q.Set("includeDeleted", fmt.Sprint(includeDeleted))
		return closeRead(c, listOrg, "/orgs/{org}/rolled-up-je-configurations", q)
	}}
	get := &cobra.Command{Use: "get CONFIG_ID", Short: "Get one rolled-up JE configuration", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, a []string) error {
		return closeRead(c, getOrg, "/orgs/{org}/rolled-up-je-configurations/"+url.PathEscape(a[0]), nil)
	}}
	addCloseReadFlags(list, &listOrg)
	addCloseReadFlags(get, &getOrg)
	list.Flags().BoolVar(&activeOnly, "active-only", true, "Only return active configurations")
	list.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include deleted configurations")
	cmd.AddCommand(list, get)
	return cmd
}

type closeMutationFlags struct {
	orgID, input, eTag string
	yes, dryRun        bool
}
type closeMutationSpec struct {
	use, short, long, operation, method, path, inputHelp string
	pathFromArg                                          func([]string) string
	inputRequired, eTag                                  bool
}

func newCloseJSONMutationCmd(spec closeMutationSpec) *cobra.Command {
	var f closeMutationFlags
	cmd := &cobra.Command{Use: spec.use, Short: spec.short, Long: spec.long, Args: func(cmd *cobra.Command, args []string) error {
		expected := len(strings.Fields(spec.use)) - 1
		if len(args) != expected {
			return fmt.Errorf("accepts %d arg(s), received %d", expected, len(args))
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		path := spec.path
		if spec.pathFromArg != nil {
			path = spec.pathFromArg(args)
		}
		var body any
		if f.input != "" {
			raw, err := readJSONObject(f.input, cmd.InOrStdin())
			if err != nil {
				return err
			}
			body = raw
		} else if spec.inputRequired {
			return errors.New("--input is required")
		} else if spec.method == http.MethodPost {
			// The dashboard sends an empty JSON object for body-optional POSTs
			// (notably task run). Preserve that wire contract rather than sending
			// a zero-byte body that some HTTP decoders treat as malformed.
			body = map[string]any{}
		}
		headers := http.Header{}
		if f.eTag != "" {
			headers.Set("If-Match", f.eTag)
		}
		return closeMutation(cmd, f, spec.operation, spec.method, path, body, headers)
	}}
	addCloseMutationFlags(cmd, &f)
	cmd.Flags().StringVarP(&f.input, "input", "i", "", spec.inputHelp+" file, or - for stdin")
	if spec.eTag {
		cmd.Flags().StringVar(&f.eTag, "etag", "", "If-Match value from the latest server read")
	}
	return cmd
}

func addCloseReadFlags(cmd *cobra.Command, orgID *string) {
	cmd.Flags().StringVar(orgID, "org", "", "Organization ID override")
	cmd.Flags().Bool("json", true, "Emit machine-readable JSON (the only supported format)")
}

func addCloseMutationFlags(cmd *cobra.Command, f *closeMutationFlags) {
	cmd.Flags().StringVar(&f.orgID, "org", "", "Organization ID override")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "Confirm the organization mutation")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "Print the exact request without changing the organization")
	cmd.Flags().Bool("json", true, "Emit machine-readable JSON (the only supported format)")
}

func closeRead(cmd *cobra.Command, orgID, path string, query url.Values) error {
	resolved, err := resolveReportOrg(orgID)
	if err != nil {
		return err
	}
	path = strings.ReplaceAll(path, "{org}", url.PathEscape(resolved))
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	client := orgreports.New(resolveCoreBaseURL(), makeOrgTokenResolver(resolved))
	data, err := client.RawRequest(cmd.Context(), orgreports.APIServiceCore, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return writeAPIResponse(cmd, "", data)
}

func closeMutation(cmd *cobra.Command, f closeMutationFlags, operation, method, path string, body any, headers http.Header) error {
	orgID, err := resolveReportOrg(f.orgID)
	if err != nil {
		return err
	}
	return closeMutationResolved(cmd, f, orgID, operation, orgreports.APIServiceCore, method, path, body, headers)
}

func closeMutationResolved(cmd *cobra.Command, f closeMutationFlags, orgID, operation, service, method, path string, body any, headers http.Header) error {
	path = strings.ReplaceAll(path, "{org}", url.PathEscape(orgID))
	client := orgreports.New(resolveCoreBaseURL(), makeOrgTokenResolver(orgID))
	endpoint, err := client.RawEndpoint(service, path)
	if err != nil {
		return err
	}
	preview := map[string]any{"method": method, "url": endpoint}
	if body != nil {
		preview["body"] = body
	}
	if len(headers) > 0 {
		preview["headers"] = headers
	}
	if f.dryRun {
		return writeJSON(cmd.OutOrStdout(), mutationEnvelope{SchemaVersion: "1", Status: "preview", Operation: operation, Organization: orgID, DryRun: true, Request: preview})
	}
	if !f.yes {
		return errors.New("refusing to change the organization without --yes (use --dry-run to preview)")
	}
	var raw []byte
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	data, err := client.RawRequestBytes(cmd.Context(), service, method, path, raw, headers)
	if err != nil {
		return err
	}
	return writeAPIResponse(cmd, "", data)
}

func closeRunPath(runID string) string { return "/v3/orgs/{org}/checklists/" + url.PathEscape(runID) }
func closeTaskPath(runID, taskID string) string {
	return closeRunPath(runID) + "/tasks/" + url.PathEscape(taskID)
}
func closeTemplatePath(id string) string {
	if id == "" {
		return "/v3/orgs/{org}/checklists_templates"
	}
	return "/v3/orgs/{org}/checklists_templates/" + url.PathEscape(id)
}
func closeAssignmentBase(runID string) string {
	return "/orgs/{org}/checklist-runs/" + url.PathEscape(runID) + "/sftp-path-assignments"
}
func addQuery(values url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		values.Set(key, strings.TrimSpace(value))
	}
}
