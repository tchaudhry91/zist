#!/bin/bash
#
# setup-history-sync.sh - Set up history file sync directory
#
# This script moves ZSH history files to ~/.histories/<hostname>/ and creates
# symlinks from the original locations. This allows syncing ~/.histories/
# across machines while keeping apps working with their expected paths.
#
# Usage: ./setup-history-sync.sh [history_file...]
#
# If no files specified, defaults to ~/.zsh_history
#
# Example:
#   ./setup-history-sync.sh
#   ./setup-history-sync.sh ~/.zsh_history ~/.claude/claude_zsh_history
#

set -e

HOSTNAME=$(hostname)
HISTORIES_DIR="$HOME/.histories/$HOSTNAME"

# Default to ~/.zsh_history if no args
if [ $# -eq 0 ]; then
    set -- "$HOME/.zsh_history"
fi

echo "Setting up history sync for host: $HOSTNAME"
echo "Target directory: $HISTORIES_DIR"
echo ""

# Create target directory
mkdir -p "$HISTORIES_DIR"

for src in "$@"; do
    # Expand path
    src=$(eval echo "$src")

    if [ ! -e "$src" ]; then
        echo "SKIP: $src (does not exist)"
        continue
    fi

    if [ -L "$src" ]; then
        echo "SKIP: $src (already a symlink)"
        continue
    fi

    # Get just the filename
    filename=$(basename "$src")
    dest="$HISTORIES_DIR/$filename"

    if [ -e "$dest" ]; then
        echo "SKIP: $dest already exists"
        continue
    fi

    echo "Moving: $src -> $dest"
    mv "$src" "$dest"

    echo "Linking: $src -> $dest"
    ln -s "$dest" "$src"

    echo "  Done"
    echo ""
done

echo "Setup complete. Directory structure:"
ls -la "$HISTORIES_DIR"
echo ""
echo "Symlinks:"
for src in "$@"; do
    src=$(eval echo "$src")
    if [ -L "$src" ]; then
        ls -la "$src"
    fi
done
echo ""
echo "You can now sync ~/.histories/ to other machines."
echo "Then run: zist collect ~/.histories/"
