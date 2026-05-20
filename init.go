package main

// zshInit is printed by `hop init zsh`. The user adds this to ~/.zshrc:
//   eval "$(hop init zsh)"
//
// The function intercepts zero-arg invocations to capture the selected path
// and cd to it. All other invocations pass straight to the binary.
const zshInit = `# hop — filesystem bookmark navigator shell integration
hop() {
  if [[ $# -eq 0 ]]; then
    local __hop_result
    __hop_result="$(command hop --pick)"
    [[ -n "$__hop_result" ]] && builtin cd "$__hop_result"
  else
    command hop "$@"
  fi
}
`
