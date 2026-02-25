package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dwellir-public/cli/internal/api"
)

const defaultDocsBaseURL = "https://www.dwellir.com/docs"

var (
	docsLimit int
	docsAll   bool
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Search and fetch Dwellir documentation",
	Long:  "Search the public docs index and fetch markdown pages from https://www.dwellir.com/docs.",
}

var docsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List documentation pages",
	RunE: func(cmd *cobra.Command, args []string) error {
		docsClient := newDocsAPI()
		entries, err := docsClient.List()
		if err != nil {
			return getFormatter().Error("docs_unavailable", "Unable to fetch docs index.", err.Error())
		}

		if !docsAll && docsLimit > 0 && len(entries) > docsLimit {
			entries = entries[:docsLimit]
		}
		return getFormatter().Success("docs.list", entries)
	},
}

var docsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search docs pages by title, slug, and description",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		docsClient := newDocsAPI()
		entries, err := docsClient.Search(args[0], docsLimit)
		if err != nil {
			return getFormatter().Error("docs_unavailable", "Unable to search docs index.", err.Error())
		}
		if len(entries) == 0 {
			return getFormatter().Error("not_found", fmt.Sprintf("No docs pages matched %q.", args[0]), "Run 'dwellir docs list' to browse available pages.")
		}
		return getFormatter().Success("docs.search", entries)
	},
}

var docsGetCmd = &cobra.Command{
	Use:   "get <slug-or-url>",
	Short: "Fetch a docs page as markdown",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		docsClient := newDocsAPI()
		page, err := docsClient.Get(args[0])
		if err != nil {
			if errors.Is(err, api.ErrDocsPageNotFound) {
				return getFormatter().Error("not_found", fmt.Sprintf("Docs page %q was not found.", args[0]), "Run 'dwellir docs search <query>' to find a valid page.")
			}
			return getFormatter().Error("docs_unavailable", "Unable to fetch docs page.", err.Error())
		}
		return getFormatter().Success("docs.get", page)
	},
}

func newDocsAPI() *api.DocsAPI {
	docsBaseURL := strings.TrimSpace(os.Getenv("DWELLIR_DOCS_BASE_URL"))
	if docsBaseURL == "" {
		docsBaseURL = defaultDocsBaseURL
	}

	indexURL := strings.TrimSpace(os.Getenv("DWELLIR_DOCS_INDEX_URL"))
	if indexURL == "" {
		indexURL = strings.TrimSuffix(docsBaseURL, "/") + "/llms.txt"
	}

	return api.NewDocsAPIWithURLs(indexURL, docsBaseURL, nil)
}

func init() {
	docsListCmd.Flags().IntVar(&docsLimit, "limit", 25, "Maximum number of results to return")
	docsListCmd.Flags().BoolVar(&docsAll, "all", false, "Return all docs pages")
	docsSearchCmd.Flags().IntVar(&docsLimit, "limit", 10, "Maximum number of results to return")

	docsCmd.AddCommand(docsListCmd, docsSearchCmd, docsGetCmd)
	rootCmd.AddCommand(docsCmd)
}
