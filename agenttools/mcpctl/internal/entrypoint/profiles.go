package entrypoint

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/syunkitada/myaitoolbox/mcpctl/internal/application"
	infraProfile "github.com/syunkitada/myaitoolbox/mcpctl/internal/infrastructure/profile"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "Manage profiles",
	Run: func(cmd *cobra.Command, args []string) {
		resolver := infraProfile.NewResolver()
		out, err := application.ListProfiles(resolver)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		if out != "" {
			fmt.Println(out)
		}
	},
}

var currentProfileCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current profile",
	Run: func(cmd *cobra.Command, args []string) {
		resolver := infraProfile.NewResolver()
		name, err := application.GetCurrentProfile(resolver, profileFlag)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(name)
	},
}

var useProfileCmd = &cobra.Command{
	Use:   "use [profile_name]",
	Short: "Change default profile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		resolver := infraProfile.NewResolver()
		out, err := application.UseProfile(resolver, args[0])
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println(out)
	},
}

func init() {
	RootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(currentProfileCmd)
	profilesCmd.AddCommand(useProfileCmd)
}
