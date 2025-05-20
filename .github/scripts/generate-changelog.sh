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
RELEASE_TAG="${RELEASE_TAG:-${GITHUB_REF_NAME:-$(git describe --tags --abbrev=0 2>/dev/null || echo "")}}"
PREVIOUS_TAG="${PREVIOUS_TAG:-$(git describe --tags --abbrev=0 "$RELEASE_TAG"^ 2>/dev/null || echo "")}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-berrybytes/awsctl}"

# Cleanup temporary file on exit
trap 'rm -f "$TEMP_FILE"' EXIT

format_commit_message() {
  local msg="$1"
  msg=$(echo "$msg" | sed -E 's/\[[^]]*\]//g')

  msg=$(echo "$msg" | sed -E '
    s/^(feat|fix|perf|refactor|docs|style|chore|test|build|ci|revert)(\([^)]*\))?:[[:space:]]*//i;
    s/^[[:space:]]+//;
    s/[[:space:]]+$//;
  ')

  msg=$(echo "$msg" | sed -E '
    s/^./\U&/;
    s/\.$//;
  ')

  echo "$msg"
}

# Generate changelog content
generate_changelog_content() {
  local EXCLUDE_PATTERNS="^docs(?!.*readme)|^test|^chore(?!.*golangci)|^ci|^build|\brelease\b|\bworkflow\b|\bchangelog\b|^style|^refactor|^wip|^merge"
  local INTERNAL_PATTERNS="\[internal\]|\[ci\]|\[wip\]|\[skip ci\]|\[release\]"

  if ! git describe --exact-match "$RELEASE_TAG" >/dev/null 2>&1; then
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" HEAD | grep -vE "$INTERNAL_PATTERNS" | \
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$PREVIOUS_TAG..HEAD" | grep -vE "$INTERNAL_PATTERNS" | \
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    fi
  else
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$RELEASE_TAG" | grep -vE "$INTERNAL_PATTERNS" | \
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$PREVIOUS_TAG..$RELEASE_TAG" | grep -vE "$INTERNAL_PATTERNS" | \
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    fi
  fi

  echo ""
  echo "Generated on $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}

# Main execution
main() {
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

  if [ -n "${GITHUB_ACTIONS:-}" ] && [ -n "${GITHUB_TOKEN:-}" ]; then
    if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
      gh release edit "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE"
    else
      gh release create "$RELEASE_TAG" --notes-file "$CHANGELOG_FILE" --title "$RELEASE_TAG"
    fi
  fi

  echo "Changelog generated successfully at $CHANGELOG_FILE"
}

main "$@"
