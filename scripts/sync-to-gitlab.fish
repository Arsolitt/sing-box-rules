#!/usr/bin/env fish

set --local REMOTE_NAME gitlab
set --local dry_run false
set --local errors 0

argparse 'dry-run' -- $argv
or begin
    echo "Usage: fish sync-to-gitlab.fish [gitlab-url] [--dry-run]"
    exit 1
end

if set --query _flag_dry_run
    set dry_run true
end

if not git rev-parse --is-inside-work-tree >/dev/null 2>&1
    echo "Error: not inside a git repository."
    exit 1
end

if not git diff --quiet 2>/dev/null; or not git diff --cached --quiet 2>/dev/null
    echo "Warning: uncommitted changes detected. Commit or stash before syncing."
    exit 1
end

echo "Fetching from origin..."
git fetch origin --prune

set --local existing_url (git remote get-url $REMOTE_NAME 2>/dev/null)
if test $status -eq 0
    if test (count $argv) -gt 0; and test "$argv[1]" != "$existing_url"
        echo "Updating remote '$REMOTE_NAME' URL: $argv[1]"
        git remote set-url $REMOTE_NAME $argv[1]
    end
else if test (count $argv) -gt 0
    echo "Adding remote '$REMOTE_NAME': $argv[1]"
    git remote add $REMOTE_NAME $argv[1]
else
    echo "Error: remote '$REMOTE_NAME' not found. Provide URL: fish sync-to-gitlab.fish <gitlab-url>"
    exit 1
end

set --local push_args
if test "$dry_run" = true
    set push_args --dry-run
end

# Collect branches from origin (skip symbolic refs like refs/remotes/origin/HEAD).
set --local branches
for ref in (git for-each-ref --format='%(refname)' refs/remotes/origin/)
    test "$ref" = refs/remotes/origin/HEAD; and continue
    set --append branches (string replace 'refs/remotes/origin/' '' $ref)
end

set --local current_branch (git rev-parse --abbrev-ref HEAD 2>/dev/null)
set --local branches_pushed 0

for branch in $branches
    echo ""
    echo "=== $branch ==="

    # Switch to the branch; create it from origin if it doesn't exist locally.
    if git show-ref --verify --quiet refs/heads/$branch
        git checkout $branch
    else
        echo "Creating local branch from origin/$branch"
        git checkout -b $branch origin/$branch
    end
    if test $status -ne 0
        echo "Error: failed to switch to '$branch'"
        set errors (math $errors + 1)
        continue
    end

    echo "Pulling from origin..."
    git pull --ff-only origin $branch
    if test $status -ne 0
        echo "Warning: pull for '$branch' did not fast-forward (diverged?)"
    end

    echo "Pushing to $REMOTE_NAME..."
    git push $REMOTE_NAME $branch $push_args
    if test $status -ne 0
        echo "Error: failed to push branch '$branch'"
        set errors (math $errors + 1)
    else
        set branches_pushed (math $branches_pushed + 1)
    end
end

# Return to the branch we started on.
if test -n "$current_branch"
    git checkout $current_branch >/dev/null 2>&1
end

set --local tags (git for-each-ref --format='%(refname:short)' refs/tags/)
set --local tags_pushed 0

for tag in $tags
    echo "Pushing tag: $tag"
    git push $REMOTE_NAME $tag $push_args
    if test $status -ne 0
        echo "Error: failed to push tag '$tag'"
        set errors (math $errors + 1)
    else
        set tags_pushed (math $tags_pushed + 1)
    end
end

echo ""
echo "--- Summary ---"
echo "Branches: $branches_pushed/"(count $branches)" pushed"
echo "Tags:     $tags_pushed/"(count $tags)" pushed"

if test $errors -gt 0
    echo "Errors:   $errors"
    exit 1
end

echo "Done."
exit 0
