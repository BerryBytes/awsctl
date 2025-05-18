#!/usr/bin/env bash
# generate-changelog.sh
# Generates a changelog between the latest and previous Git tags.
# - Includes commit messages, excluding docs, test, and chore commits
# - Updates `CHANGELOG.md` in the repo root
# - Formats the changelog using Prettier (if available)
# - Updates GitHub release notes when run in GitHub Actions
set -euo pipefail

# Configuration
PROJECT_NAME="awsctl"
CHANGELOG_FILE="CHANGELOG.md"
TEMP_FILE=".tmpchangelog"
RELEASE_TAG="${RELEASE_TAG:-${GITHUB_REF_NAME:-$(git describe --tags --abbrev=0)}}"
PREVIOUS_TAG="${PREVIOUS_TAG:-$(git describe --tags --abbrev=0 "$RELEASE_TAG"^ 2>/dev/null || echo "")}"

# Cleanup temporary file on exit
trap 'rm -f "$TEMP_FILE"' EXIT

# Ensure dependencies are present
check_dependencies() {
  for cmd in git gh; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "Error: $cmd is not installed"
      exit 1
    fi
  done
}

# Format commit messages for better readability
format_commit_message() {
  local msg="$1"
  # Remove common prefixes and capitalize first letter
  msg="${msg//^feat:/}"
  msg="${msg//^fix:/}"
  msg="$(echo "$msg" | sed -E 's/^[[:space:]]*//; s/^./\U&/')"
  echo "$msg"
}

# Generate changelog content
generate_changelog_content() {
  # For first release or if RELEASE_TAG is not yet fully committed, use HEAD
  if ! git describe --exact-match "$RELEASE_TAG" >/dev/null 2>&1; then
    echo "Warning: Tag $RELEASE_TAG does not exist. Using HEAD for changelog."
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="^docs\|^test\|^chore" \
        --pretty=format:"- %s (%h)" HEAD | while read -r line; do
        format_commit_message "$line"
      done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="^docs\|^test\|^chore" \
        --pretty=format:"- %s (%h)" "$PREVIOUS_TAG..HEAD" | while read -r line; do
        format_commit_message "$line"
      done
    fi
  else
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="^docs\|^test\|^chore" \
        --pretty=format:"- %s (%h)" "$RELEASE_TAG" | while read -r line; do
        format_commit_message "$line"
      done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="^docs\|^test\|^chore" \
        --pretty=format:"- %s (%h)" "$PREVIOUS_TAG..$RELEASE_TAG" | while read -r line; do
        format_commit_message "$line"
      done
    fi
  fi

  echo ""
  echo "Generated on $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}

# Main execution
main() {
  check_dependencies

  # Create or update changelog
  if [ -f "$CHANGELOG_FILE" ]; then
    echo "Updating existing changelog..."
    generate_changelog_content > "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    cat "$CHANGELOG_FILE" >> "$TEMP_FILE"
    mv "$TEMP_FILE" "$CHANGELOG_FILE"
  else
    echo "Creating new changelog..."
    generate_changelog_content > "$CHANGELOG_FILE"
  fi

  # Format the changelog if Prettier is available
  if [ -f "$CHANGELOG_FILE" ] && command -v prettier >/dev/null 2>&1; then
    prettier --write "$CHANGELOG_FILE"
  else
    echo "Note: Prettier is not installed. Skipping changelog formatting."
  fi

  # Create GitHub release notes if running in GitHub Actions with token available
  if [ -n "${GITHUB_ACTIONS:-}" ] && [ -n "${GITHUB_TOKEN:-}" ]; then
    if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
      gh release edit "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE"
    else
      gh release create "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE" --title "$RELEASE_TAG"
    fi
  else
    echo "Note: Not running in GitHub Actions or GITHUB_TOKEN not set. Skipping GitHub release notes update."
  fi

  echo "Changelog generated successfully at $CHANGELOG_FILE"
}

main "$@"
