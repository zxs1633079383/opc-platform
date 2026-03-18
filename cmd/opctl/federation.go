package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/federation"
)

var federationCmd = &cobra.Command{
	Use:   "federation",
	Short: "Manage OPC federation (multi-company)",
}

// --- init subcommand ---

var federationInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the federation controller",
	RunE:  runFederationInit,
}

var federationInitName string

func init() {
	federationInitCmd.Flags().StringVar(&federationInitName, "name", "default", "federation name")

	federationCmd.AddCommand(federationInitCmd)
	federationCmd.AddCommand(federationAddCompanyCmd)
	federationCmd.AddCommand(federationCompaniesCmd)
	federationCmd.AddCommand(federationStatusCmd)

	rootCmd.AddCommand(federationCmd)
}

func runFederationInit(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	_ = federation.NewController(logger)
	fmt.Printf("Federation %q initialized.\n", federationInitName)
	return nil
}

// --- add-company subcommand ---

var federationAddCompanyCmd = &cobra.Command{
	Use:   "add-company",
	Short: "Register a company to the federation",
	RunE:  runFederationAddCompany,
}

var (
	addCompanyName     string
	addCompanyEndpoint string
	addCompanyType     string
)

func init() {
	federationAddCompanyCmd.Flags().StringVar(&addCompanyName, "name", "", "company name (required)")
	federationAddCompanyCmd.Flags().StringVar(&addCompanyEndpoint, "endpoint", "", "company endpoint URL (required)")
	federationAddCompanyCmd.Flags().StringVar(&addCompanyType, "type", "software", "company type (software|operations|sales|custom)")
	federationAddCompanyCmd.MarkFlagRequired("name")
	federationAddCompanyCmd.MarkFlagRequired("endpoint")
}

func runFederationAddCompany(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	company, err := fc.RegisterCompany(federation.CompanyRegistration{
		Name:     addCompanyName,
		Endpoint: addCompanyEndpoint,
		Type:     federation.CompanyType(addCompanyType),
	})
	if err != nil {
		return fmt.Errorf("add company: %w", err)
	}

	fmt.Printf("Company registered: id=%s name=%s type=%s endpoint=%s\n",
		company.ID, company.Name, company.Type, company.Endpoint)
	return nil
}

// --- companies subcommand ---

var federationCompaniesCmd = &cobra.Command{
	Use:   "companies",
	Short: "List all federated companies",
	RunE:  runFederationCompanies,
}

func runFederationCompanies(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	companies := fc.ListCompanies()

	if len(companies) == 0 {
		fmt.Println("No companies registered.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tSTATUS\tENDPOINT\tAGENTS")
	for _, c := range companies {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			c.ID, c.Name, c.Type, c.Status, c.Endpoint, len(c.Agents))
	}
	return w.Flush()
}

// --- status subcommand ---

var federationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show federation status",
	RunE:  runFederationStatus,
}

func runFederationStatus(cmd *cobra.Command, args []string) error {
	logger := ensureLogger()

	fc := federation.NewController(logger)
	companies := fc.ListCompanies()

	fmt.Println("OPC Federation Status")
	fmt.Println("====================")

	if len(companies) == 0 {
		fmt.Println("\nNo companies registered.")
		fmt.Println("Run 'opctl federation add-company' to register a company.")
		return nil
	}

	var online, offline, busy int
	var totalAgents int
	for _, c := range companies {
		switch c.Status {
		case federation.CompanyStatusOnline:
			online++
		case federation.CompanyStatusOffline:
			offline++
		case federation.CompanyStatusBusy:
			busy++
		}
		totalAgents += len(c.Agents)
	}

	fmt.Printf("\nCompanies: %d total (%d online, %d offline, %d busy)\n",
		len(companies), online, offline, busy)
	fmt.Printf("Agents:    %d total\n", totalAgents)

	return nil
}

// --- helper ---

func ensureLogger() *zap.SugaredLogger {
	logger := config.Logger
	if logger == nil {
		config.InitLogger(false, "")
		logger = config.Logger
	}
	return logger
}
