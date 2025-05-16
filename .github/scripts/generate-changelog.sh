#!/usr/bin/env bash
# generate-changelog.sh
# Generates a changelog between the latest and previous Git tags.
# - Includes commit messages, excluding certain commit types (e.g., docs, test).
# - Updates `CHANGELOG.md` in the repo root.
# - Formats the changelog using Prettier (if available).
# - Updates GitHub release notes when run in GitHub Actions.
set -euo pipefail

# Configuration
PROJECT_NAME="awsctl"
CHANGELOG_FILE="CHANGELOG.md"
TEMP_FILE=".tmpchangelog"
RELEASE_TAG="${RELEASE_TAG:-${GITHUB_REF_NAME:-$(git describe --tags --abbrev=0)}}"
PREVIOUS_TAG="${PREVIOUS_TAG:-$(git describe --tags --abbrev=0 "$RELEASE_TAG"^ 2>/dev/null || echo "")}"

# Ensure dependencies are present
check_dependencies() {
  for cmd in git gh; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      echo "Error: $cmd is not installed"
      exit 1
    fi
  done
}

# Generate changelog content
generate_changelog_content() {
  release_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  if [ -z "$PREVIOUS_TAG" ] || [ "$PREVIOUS_TAG" = "$RELEASE_TAG" ]; then
    echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG ($release_date)"
    git log --pretty=format:"- %s (%h)" "$RELEASE_TAG" \
      | grep -Ei '^(feat|fix|perf|style|ci|build|revert):|^[^-:]+$'
  else
    echo "# $PROJECT_NAME - $RELEASE_TAG ($release_date)"
    echo "## Changes since $PREVIOUS_TAG"
    git log --pretty=format:"- %s (%h)" "$PREVIOUS_TAG..$RELEASE_TAG" \
      | grep -Ei '^(feat|fix|perf|style|ci|build|revert):|^[^-:]+$'
  fi

  echo ""
}

# Main execution
main() {
  check_dependencies

  # Create or update changelog
  if [ -f "$CHANGELOG_FILE" ]; then
    echo "Updating existing changelog..."
    generate_changelog_content >"$TEMP_FILE"
    echo "" >>"$TEMP_FILE"
    cat "$CHANGELOG_FILE" >>"$TEMP_FILE"
    mv "$TEMP_FILE" "$CHANGELOG_FILE"
  else
    echo "Creating new changelog..."
    generate_changelog_content >"$CHANGELOG_FILE"
  fi

  # Format and validate the changelog
  if command -v prettier >/dev/null 2>&1; then
    prettier --write "$CHANGELOG_FILE"
  else
    echo "Note: Prettier is not installed. Skipping changelog formatting."
  fi

  # Create GitHub release notes if running in GitHub Actions
  if [ -n "${GITHUB_ACTIONS:-}" ]; then
    if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
      gh release edit "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE"
    else
      gh release create "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE" --title "$RELEASE_TAG"
    fi
  else
    echo "Note: Not running in GitHub Actions. Skipping GitHub release notes update."
  fi

  echo "Changelog generated successfully at $CHANGELOG_FILE"
}

main "$@"
