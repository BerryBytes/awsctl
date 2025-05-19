#!/bin/bash
set -e
set -eo pipefail

print_error_message() {
  echo "An error occurred. Please visit <link_needed> for assistance."
}

trap 'if [ $? -ne 0 ]; then print_error_message; fi' EXIT

awsctlpath="$HOME/.awsctl/"
printf "\\e[1mINSTALLATION\\e[0m\\n"
if [ -d "$awsctlpath" ]; then
  echo "Previous installation detected."
else
  mkdir "${awsctlpath}"
  echo "Created $awsctlpath folder."
fi

printf "Downloading awsctl."
print_dots() {
  while true; do
    printf "."
    sleep 1
  done
}
print_dots &

ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
elif [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
else
  echo "Unsupported architecture: $ARCH"
  exit 1
fi

OS_NAME=$(uname -s)
VERSION=${1:-latest} # Accept version as an argument, default to 'latest' if not provided
if [ "$OS_NAME" = "Darwin" ]; then
  if curl -sS -L "https://github.com/berrybytes/awsctl/releases/download/${VERSION}/awsctl_Darwin_${ARCH}" -o "$HOME/.awsctl/awsctl"; then
    kill "$!" 2>/dev/null
    echo ""
    echo "Download completed"
  else
    kill "$!" 2>/dev/null
    echo "Downloading failed. Check your internet connection."
    exit 1
  fi
elif [ "$OS_NAME" = "Linux" ]; then
  if curl -sS -L "https://github.com/berrybytes/awsctl/releases/download/${VERSION}/awsctl_Linux_${ARCH}" -o "$HOME/.awsctl/awsctl"; then
    kill "$!" 2>/dev/null
    echo ""
    echo "Download completed"
  else
    kill "$!" 2>/dev/null
    echo "Downloading failed. Check your internet connection."
    exit 1
  fi
else
  kill "$!" 2>/dev/null
  echo "Direct installer for $OS_NAME is not yet supported: Check GITHUB RELEASES for manual installation"
  echo "RELEASES: https://github.com/berrybytes/awsctl/releases"
  exit 1
fi

chmod +x "$HOME/.awsctl/awsctl"
if [ $? -eq 0 ]; then
  echo ""
else
  echo "Error: chmod failed."
  exit 1
fi

CURRENT_SHELL="$SHELL"

if [[ "$CURRENT_SHELL" = "/bin/bash" || "$CURRENT_SHELL" = "/usr/bin/bash" ]]; then
  echo "Detected Bash shell."
  CONFIG_FILE="$HOME/.bashrc"
  if grep -q ".awsctl" "$CONFIG_FILE"; then
    echo "The PATH is already set in $CONFIG_FILE."
  else
    echo "export PATH=\"$HOME/.awsctl:\$PATH\"" >>"$CONFIG_FILE"
    source "$CONFIG_FILE"
  fi
elif [[ "$CURRENT_SHELL" = "/bin/zsh" || "$CURRENT_SHELL" = "/usr/bin/zsh" ]]; then
  echo "Detected Zsh shell."
  CONFIG_FILE="$HOME/.zshrc"
  if grep -q ".awsctl" "$CONFIG_FILE"; then
    echo "The PATH is already set in $CONFIG_FILE."
  else
    echo "export PATH=\"$HOME/.awsctl:\$PATH\"" >>"$CONFIG_FILE"
    echo ""
    zsh
  fi
elif [[ "$CURRENT_SHELL" = "/bin/fish" || "$CURRENT_SHELL" = "/usr/bin/fish" ]]; then
  echo "Detected Fish shell."
  fish -c "set -U fish_user_paths \"$HOME/.awsctl\" \$fish_user_paths"
else
  printf "\\e[1mFAILURE\\e[0m\\n"
  echo "Unsupported shell detected: $CURRENT_SHELL"
  echo "Please set the PATH manually."
  echo "See <link_needed>"
fi

echo ""
printf "\\e[1m------INSTALLATION COMPLETED------\\e[0m\\n"
echo ""
printf "\\e[1mSUMMARY\\e[0m\\n"
echo "    awsctl is a CLI tool for managing AWS resources."
echo ""
printf "\\e[1mUSAGE\\e[0m\\n"
echo "    awsctl --help"
echo ""
printf "\\e[1mUNINSTALL\\e[0m\\n"
echo "    everything is installed into ~/.awsctl/,"
echo "    so you can remove it like so:"
echo ""
echo "    rm -rf ~/.awsctl/"
echo ""
printf "\\e[1mTIP\\e[0m\\n"
printf "    Inorder to use awsctl in this terminal, please run \\e[34msource ~/.bashrc\\e[0m or your own shell CONFIG_FILE\n"
echo ""
exit
