package entrypoint

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion zsh",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script.

To use with zsh, run:
  source <(mymcp completion zsh)

Or save to a file and add to your .zshrc:
  mymcp completion zsh > ~/.zsh/completions/_mymcp
  echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
  echo 'autoload -Uz compinit && compinit' >> ~/.zshrc`,
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	Run: func(cmd *cobra.Command, args []string) {
		out := os.Stdout
		_, _ = fmt.Fprint(out, zshCompletionScript)
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionZshCmd)
}

const zshCompletionScript = `#compdef mymcp

__mymcp_list_tools() {
  local -a tools
  local service="${commands[mymcp]:-mymcp}"
  tools=(${(f)"$("$service" __list_tools 2>/dev/null)"})
  _describe -t tools 'tool' tools
}

__mymcp_list_params() {
  local -a params
  local service="${commands[mymcp]:-mymcp}"
  params=(${(f)"$("$service" __list_params "$1" 2>/dev/null)"})
  _describe -t params 'parameter' params
}

__mymcp_list_param_values() {
  local -a values
  local service="${commands[mymcp]:-mymcp}"
  values=(${(f)"$("$service" __list_param_values "$1" "$2" 2>/dev/null)"})
  if (( $#values )); then
    _describe -t values "values for $2" values
  fi
}

__mymcp_servers() {
  local -a servers
  local service="${commands[mymcp]:-mymcp}"
  servers=(${(f)"$("$service" __list_tools 2>/dev/null | sed 's|/.*||' | sort -u)"})
  _describe -t servers 'server' servers
}

_mymcp() {
  local curcontext="$curcontext" ret=1

  if (( CURRENT == 2 )); then
    local -a subcommands
    subcommands=(
      'list:List available tools'
      'call:Call a tool'
      'info:Show tool information'
      'search:Search tools'
      'profiles:Manage profiles'
      'completion:Generate shell completion script'
    )
    _describe -t commands 'command' subcommands && ret=0
  else
    case $words[2] in
      list)
        if (( CURRENT == 3 )); then
          __mymcp_servers && ret=0
        fi
        ;;
      call)
        if (( CURRENT == 3 )); then
          __mymcp_list_tools && ret=0
        elif (( CURRENT > 3 )); then
          local prev=$words[$((CURRENT-1))]
          if [[ $prev == --* ]]; then
            local paramName=${prev#--}
            __mymcp_list_param_values "$words[3]" "$paramName" && ret=0
          else
            __mymcp_list_params "$words[3]" && ret=0
          fi
        fi
        ;;
      info)
        if (( CURRENT == 3 )); then
          __mymcp_list_tools && ret=0
        fi
        ;;
      profiles)
        if (( CURRENT == 3 )); then
          local -a profile_subcmds
          profile_subcmds=(
            'current:Show current profile'
            'use:Switch default profile'
          )
          _describe -t commands 'profile command' profile_subcmds && ret=0
        fi
        ;;
      completion)
        if (( CURRENT == 3 )); then
          local -a completion_subcmds
          completion_subcmds=('zsh:Generate zsh completion script')
          _describe -t commands 'completion command' completion_subcmds && ret=0
        fi
        ;;
    esac
  fi

  return ret
}

compdef _mymcp mymcp
`
