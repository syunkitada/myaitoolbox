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
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver := infraProfile.NewResolver()
		out, err := application.ListProfiles(resolver)
		if err != nil {
			return err
		}
		if out != "" {
			fmt.Println(out)
		}
		return nil
	},
}

var currentProfileCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver := infraProfile.NewResolver()
		name, err := application.GetCurrentProfile(resolver, profileFlag)
		if err != nil {
			return err
		}
		fmt.Println(name)
		return nil
	},
}

var useProfileCmd = &cobra.Command{
	Use:   "use [profile_name]",
	Short: "Change default profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver := infraProfile.NewResolver()
		out, err := application.UseProfile(resolver, args[0])
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(currentProfileCmd)
	profilesCmd.AddCommand(useProfileCmd)
}
