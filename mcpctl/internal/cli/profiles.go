package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/profile"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage profiles",
	Run: func(cmd *cobra.Command, args []string) {
		// Just listing if no subcommands
		listProfiles()
	},
}

var currentProfileCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current profile",
	Run: func(cmd *cobra.Command, args []string) {
		p, err := profile.ResolveProfile(profileFlag, "")
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(p.Name)
	},
}

var useProfileCmd = &cobra.Command{
	Use:   "use [profile_name]",
	Short: "Change default profile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		newProfile := args[0]

		cfg, err := profile.LoadConfig()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		
		oldProfile := cfg.DefaultProfile
		cfg.DefaultProfile = newProfile

		if err := profile.SaveConfig(cfg); err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Printf("default profile changed:\n  %s -> %s\n", oldProfile, newProfile)
	},
}

func listProfiles() {
	cfg, err := profile.LoadConfig()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	profs, err := profile.ListProfiles()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if len(profs) == 0 {
		fmt.Println("No profiles found in ~/.config/mcpctl/profiles/")
		return
	}

	for _, p := range profs {
		if p == cfg.DefaultProfile {
			fmt.Printf("* %s\n", p)
		} else {
			fmt.Printf("  %s\n", p)
		}
	}
}

func init() {
	RootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(currentProfileCmd)
	profilesCmd.AddCommand(useProfileCmd)
}
