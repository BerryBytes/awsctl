#!/usr/bin/env bash
# generate-changelog.sh
# Generates a changelog and release notes for awsctl.
# - Changelog includes all commits, excluding docs, test, and chore commits.
# - Release notes are user-focused, using initial or future template based on tag existence.
# - Updates `CHANGELOG.md` and `RELEASE_NOTES.md` in the repo root.
# - Formats files using Prettier (if available).
# - Updates GitHub release notes when run in GitHub Actions.
set -euo pipefail

# Configuration
PROJECT_NAME="awsctl"
CHANGELOG_FILE="CHANGELOG.md"
RELEASE_NOTES_FILE="RELEASE_NOTES.md"
TEMP_FILE=".tmpchangelog"
TEMP_RELEASE_NOTES=".tmpreleasenotes"
INITIAL_TEMPLATE=".github/initial-release-template.md"
FUTURE_TEMPLATE=".github/future-release-template.md"
RELEASE_TAG="${RELEASE_TAG:-${GITHUB_REF_NAME:-$(git describe --tags --abbrev=0 2>/dev/null || echo "")}}"
PREVIOUS_TAG="${PREVIOUS_TAG:-$(git describe --tags --abbrev=0 "$RELEASE_TAG"^ 2>/dev/null || echo "")}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-berrybytes/awsctl}"
RELEASE_DATE=$(date -u +"%B %d, %Y")

# Cleanup temporary files on exit
trap 'rm -f "$TEMP_FILE" "$TEMP_RELEASE_NOTES"' EXIT

# Format commit messages for changelog
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
  local EXCLUDE_PATTERNS="^docs\\(internal\\)|^test|^chore(?!.*golangci)|^ci|^build|^style|^refactor|^wip|^merge"
  local INTERNAL_PATTERNS="\[internal\]|\[ci\]|\[wip\]|\[skip ci\]|\[release\]"

  if ! git describe --exact-match "$RELEASE_TAG" >/dev/null 2>&1; then
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" HEAD | grep -vE "$INTERNAL_PATTERNS" |
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$PREVIOUS_TAG..HEAD" | grep -vE "$INTERNAL_PATTERNS" |
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    fi
  else
    if [ -z "$PREVIOUS_TAG" ]; then
      echo "# $PROJECT_NAME - Initial Release $RELEASE_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$RELEASE_TAG" | grep -vE "$INTERNAL_PATTERNS" |
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    else
      echo "# $PROJECT_NAME - $RELEASE_TAG"
      echo "## Changes since $PREVIOUS_TAG"
      git log --no-merges --invert-grep --grep="$EXCLUDE_PATTERNS" \
        --pretty=format:"%s|%h|%H|%an|%ae" "$PREVIOUS_TAG..$RELEASE_TAG" | grep -vE "$INTERNAL_PATTERNS" |
        while IFS='|' read -r msg short_hash full_hash author email; do
          formatted=$(format_commit_message "$msg")
          [ -n "$formatted" ] && echo "- [$short_hash](https://github.com/$GITHUB_REPOSITORY/commit/$full_hash) $formatted ($author <$email>)"
        done
    fi
  fi

  echo ""
  echo "Generated on $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
}

# Generate release notes content
generate_release_notes() {
  local template
  if [ -z "$PREVIOUS_TAG" ]; then
    template="$INITIAL_TEMPLATE"
    echo "Using initial release template for $RELEASE_TAG"
  else
    template="$FUTURE_TEMPLATE"
    echo "Using future release template for $RELEASE_TAG"
  fi

  if [ ! -f "$template" ]; then
    echo "Error: Template $template not found"
    exit 1
  fi

  cp "$template" "$TEMP_RELEASE_NOTES"
  sed -i "s/{{ .Version }}/$RELEASE_TAG/g" "$TEMP_RELEASE_NOTES"
  sed -i "s/{{ .Date }}/$RELEASE_DATE/g" "$TEMP_RELEASE_NOTES"

  # Extract features and fixes
  local features=""
  local fixes=""
  local documentation=""
  if [ -z "$PREVIOUS_TAG" ]; then
    # Initial release: Extract commits from all history up to HEAD
    while IFS='|' read -r msg short_hash full_hash author email; do
      formatted=$(format_commit_message "$msg")
      if [[ "$msg" =~ ^feat ]]; then
        [ -n "$formatted" ] && features+="- $formatted\n"
      fi
    done < <(git log --no-merges --grep='^feat' \
      --pretty=format:"%s|%h|%H|%an|%ae" HEAD)
  else
    # Future release: Extract commits between PREVIOUS_TAG and RELEASE_TAG
    while IFS='|' read -r msg short_hash full_hash author email; do
      formatted=$(format_commit_message "$msg")
      if [[ "$msg" =~ ^feat ]]; then
        [ -n "$formatted" ] && features+="- $formatted\n"
      elif [[ "$msg" =~ ^fix ]]; then
        [ -n "$formatted" ] && fixes+="- $formatted\n"
      elif [[ "$msg" =~ ^docs ]]; then
        [ -n "$formatted" ] && documentation+="- $formatted\n"
      fi
    done < <(git log --no-merges --grep='^feat\|^fix' \
      --pretty=format:"%s|%h|%H|%an|%ae" "$PREVIOUS_TAG..$RELEASE_TAG")
  fi

  # Insert features and fixes into template
  if [ -n "$features" ]; then
    if [ -z "$PREVIOUS_TAG" ]; then
      sed -i "/### Features/a $features" "$TEMP_RELEASE_NOTES"
    else
      sed -i "/### New Features/a $features" "$TEMP_RELEASE_NOTES"
    fi
  else
    if [ -z "$PREVIOUS_TAG" ]; then
      sed -i "/### Features/a - No new features in this release" "$TEMP_RELEASE_NOTES"
    else
      sed -i "/### New Features/a - No new features in this release" "$TEMP_RELEASE_NOTES"
    fi
  fi
  if [ -n "$fixes" ]; then
    sed -i "/### Bug Fixes/a $fixes" "$TEMP_RELEASE_NOTES"
  else
    sed -i "/### Bug Fixes/a - No bug fixes in this release" "$TEMP_RELEASE_NOTES"
  fi
  if [ -n "$documentation" ]; then
    sed -i "/### Documentation Update/a $documentation" "$TEMP_RELEASE_NOTES"
  else
    sed -i "/### Documentation Update/a - No documentation update in this release" "$TEMP_RELEASE_NOTES"
  fi

  mv "$TEMP_RELEASE_NOTES" "$RELEASE_NOTES_FILE"
}

# Main execution
main() {
  # Generate changelog
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

  # Generate release notes
  generate_release_notes

  # Format files if Prettier is available
  if command -v prettier >/dev/null 2>&1; then
    prettier --write "$CHANGELOG_FILE" "$RELEASE_NOTES_FILE"
  else
    echo "Note: Prettier is not installed. Skipping formatting."
  fi

  # Update GitHub release
  if [ -n "${GITHUB_ACTIONS:-}" ] && [ -n "${GITHUB_TOKEN:-}" ]; then
    if gh release view "$RELEASE_TAG" >/dev/null 2>&1; then
      gh release edit "$RELEASE_TAG" --notes-file "$RELEASE_NOTES_FILE"
    else
      gh release create "$RELEASE_TAG" --notes-file "$RELEASE_NOTES_FILE" --title "$RELEASE_TAG"
    fi
  fi

  echo "Changelog ($CHANGELOG_FILE) and release notes ($RELEASE_NOTES_FILE) generated successfully"
}

main "$@"
