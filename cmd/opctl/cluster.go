package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zlc-ai/opc-platform/internal/config"
	"github.com/zlc-ai/opc-platform/pkg/cluster"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage OPC cluster",
}

// --- init subcommand ---

var clusterInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize this node as cluster master",
	RunE:  runClusterInit,
}

var (
	initNodeID     string
	initListenAddr string
)

func init() {
	clusterInitCmd.Flags().StringVar(&initNodeID, "node-id", "", "unique node identifier (default: hostname)")
	clusterInitCmd.Flags().StringVar(&initListenAddr, "listen", "0.0.0.0:9090", "address to listen on")

	clusterCmd.AddCommand(clusterInitCmd)
	clusterCmd.AddCommand(clusterJoinCmd)
	clusterCmd.AddCommand(clusterNodesCmd)
	clusterCmd.AddCommand(clusterStatusCmd)
	clusterCmd.AddCommand(clusterLeaveCmd)

	rootCmd.AddCommand(clusterCmd)
}

func runClusterInit(cmd *cobra.Command, args []string) error {
	nodeID, err := resolveNodeID(initNodeID)
	if err != nil {
		return err
	}

	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	mgr := cluster.NewManager(logger)
	if err := mgr.Init(nodeID, initListenAddr); err != nil {
		return fmt.Errorf("cluster init: %w", err)
	}

	fmt.Printf("Cluster master initialized: node=%s listen=%s\n", nodeID, initListenAddr)
	return nil
}

// --- join subcommand ---

var clusterJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join an existing cluster",
	RunE:  runClusterJoin,
}

var (
	joinNodeID     string
	joinListenAddr string
	joinMasterAddr string
)

func init() {
	clusterJoinCmd.Flags().StringVar(&joinNodeID, "node-id", "", "unique node identifier (default: hostname)")
	clusterJoinCmd.Flags().StringVar(&joinListenAddr, "listen", "0.0.0.0:9091", "local address to listen on")
	clusterJoinCmd.Flags().StringVar(&joinMasterAddr, "master", "", "master node address (required)")
	clusterJoinCmd.MarkFlagRequired("master")
}

func runClusterJoin(cmd *cobra.Command, args []string) error {
	nodeID, err := resolveNodeID(joinNodeID)
	if err != nil {
		return err
	}

	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	mgr := cluster.NewManager(logger)
	discovery := cluster.NewHTTPDiscovery(joinMasterAddr, logger)
	mgr.SetDiscovery(discovery)

	if err := mgr.Join(nodeID, joinListenAddr, joinMasterAddr); err != nil {
		return fmt.Errorf("cluster join: %w", err)
	}

	fmt.Printf("Joined cluster: node=%s master=%s\n", nodeID, joinMasterAddr)
	return nil
}

// --- nodes subcommand ---

var clusterNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List cluster nodes",
	RunE:  runClusterNodes,
}

func runClusterNodes(cmd *cobra.Command, args []string) error {
	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	mgr := cluster.NewManager(logger)
	nodes := mgr.ListNodes()

	if len(nodes) == 0 {
		fmt.Println("No nodes in cluster.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NODE ID\tADDRESS\tROLE\tSTATUS\tAGENTS\tCPU%\tMEM%")
	for _, n := range nodes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%.1f\t%.1f\n",
			n.NodeID, n.Address, n.Role, n.Status,
			n.AgentCount, n.CPUUsage, n.MemoryUsage,
		)
	}
	return w.Flush()
}

// --- status subcommand ---

var clusterStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	RunE:  runClusterStatus,
}

func runClusterStatus(cmd *cobra.Command, args []string) error {
	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	mgr := cluster.NewManager(logger)
	nodes := mgr.ListNodes()

	local, hasLocal := mgr.LocalNode()

	fmt.Println("OPC Cluster Status")
	fmt.Println("====================")

	if !hasLocal {
		fmt.Println("\nThis node is not part of any cluster.")
		fmt.Println("Run 'opctl cluster init' or 'opctl cluster join' to get started.")
		return nil
	}

	fmt.Printf("\nLocal Node: %s (%s)\n", local.NodeID, local.Role)
	fmt.Printf("Is Master:  %v\n", mgr.IsMaster())

	var ready, notReady, leaving int
	for _, n := range nodes {
		switch n.Status {
		case cluster.NodeStatusReady:
			ready++
		case cluster.NodeStatusNotReady:
			notReady++
		case cluster.NodeStatusLeaving:
			leaving++
		}
	}

	fmt.Printf("\nNodes: %d total (%d ready, %d not-ready, %d leaving)\n",
		len(nodes), ready, notReady, leaving)

	return nil
}

// --- leave subcommand ---

var clusterLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave the cluster",
	RunE:  runClusterLeave,
}

func runClusterLeave(cmd *cobra.Command, args []string) error {
	logger := config.Logger
	if logger == nil {
		config.InitLogger(false)
		logger = config.Logger
	}

	mgr := cluster.NewManager(logger)
	if err := mgr.Leave(); err != nil {
		return fmt.Errorf("cluster leave: %w", err)
	}

	fmt.Println("Successfully left the cluster.")
	return nil
}

// --- helpers ---

func resolveNodeID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("resolve node ID: %w", err)
	}
	return hostname, nil
}
