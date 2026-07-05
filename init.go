package main

// Shell integration scripts printed by `hop init <shell>`. The user adds
//   eval "$(hop init zsh)"     to ~/.zshrc, or
//   eval "$(hop init bash)"    to ~/.bashrc, or
//   hop init fish | source     to ~/.config/fish/config.fish
//
// The function intercepts zero-arg invocations (open picker) and bare-word
// invocations (direct jump) to capture the selected path and cd to it.
// Flag and init invocations pass straight to the binary.

const zshInit = `# hop — filesystem bookmark navigator shell integration
hop() {
  local __hop_result
  if [[ $# -eq 0 ]]; then
    __hop_result="$(command hop --pick)"
    [[ -n "$__hop_result" ]] && builtin cd "$__hop_result"
  elif [[ "$1" == -* || "$1" == init ]]; then
    command hop "$@"
  else
    __hop_result="$(command hop --jump "$@")"
    [[ -n "$__hop_result" ]] && builtin cd "$__hop_result"
  fi
}
if (( $+functions[compdef] )); then
  _hop() {
    local -a __hop_labels
    __hop_labels=(${(f)"$(command hop --labels 2>/dev/null)"})
    compadd -a __hop_labels
  }
  compdef _hop hop
fi
`

const bashInit = `# hop — filesystem bookmark navigator shell integration
hop() {
  local __hop_result
  if [ $# -eq 0 ]; then
    __hop_result="$(command hop --pick)"
    [ -n "$__hop_result" ] && builtin cd "$__hop_result"
  else
    case "$1" in
      -*|init) command hop "$@" ;;
      *)
        __hop_result="$(command hop --jump "$@")"
        [ -n "$__hop_result" ] && builtin cd "$__hop_result"
        ;;
    esac
  fi
}
_hop_completions() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  COMPREPLY=($(compgen -W "$(command hop --labels 2>/dev/null)" -- "$cur"))
}
complete -F _hop_completions hop
`

const fishInit = `# hop — filesystem bookmark navigator shell integration
function hop
    if test (count $argv) -eq 0
        set -l __hop_result (command hop --pick)
        and test -n "$__hop_result"
        and cd "$__hop_result"
    else if string match -q -- '-*' $argv[1]; or test "$argv[1]" = init
        command hop $argv
    else
        set -l __hop_result (command hop --jump $argv)
        and test -n "$__hop_result"
        and cd "$__hop_result"
    end
end
complete -c hop -f -a "(command hop --labels 2>/dev/null)"
`
