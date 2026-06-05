if [[ ":$PATH:" != *":$(pwd)/bin:"* ]]; then
  export PATH="$(pwd)/bin:$PATH"
fi
source <(harness completion zsh)
