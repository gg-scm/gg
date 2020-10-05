#!/bin/bash

# Copyright 2019 The gg Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Bash completion docs:
# https://www.gnu.org/software/bash/manual/bash.html#Programmable-Completion
# https://eli.thegreenplace.net/2013/12/26/adding-bash-completion-for-your-own-tools-an-example-for-pss

_gg_complete() {
  local curr_word prev_word subcmd_idx subcmd
  curr_word="${COMP_WORDS[COMP_CWORD]}"
  prev_word="${COMP_WORDS[COMP_CWORD-1]}"
  if [[ "$prev_word" == '=' ]]; then
    # In '-foo=bar', use '-foo' as prev_word.
    prev_word="${COMP_WORDS[COMP_CWORD-2]}"
  elif [[ "$curr_word" == '=' ]]; then
    # In '-foo=', use '' as curr_word.
    curr_word=''
  fi
  # TODO(soon): Skip global options.
  subcmd_idx=1
  subcmd="${COMP_WORDS[subcmd_idx]}"

  if [[ $COMP_CWORD -eq $subcmd_idx && "$curr_word" != -* ]]; then
    local commands=( \
      add \
      backout \
      branch \
      check \
      checkout \
      ci \
      clone \
      co \
      commit \
      diff \
      evolve \
      gerrithook \
      histedit \
      history \
      id \
      identify \
      init \
      log \
      mail \
      merge \
      pr \
      pull \
      push \
      rebase \
      remove \
      rm \
      requestpull \
      revert \
      st \
      status \
      up \
      update \
      upstream \
    )
    COMPREPLY=( $(compgen -W "${commands[*]}" -- "$curr_word") )
    return 0
  fi

  named_revs() {
    local refs=( $(git show-ref --head | sed -e 's/^\S\+ //') )
    printf '%s\n' "${refs[@]}"
    printf '%s\n' "${refs[@]}" | grep '^refs/heads/' | sed -e 's:^refs/heads/::'
    printf '%s\n' "${refs[@]}" | grep '^refs/tags/' | sed -e 's:^refs/tags/::'
    printf '%s\n' "${refs[@]}" | grep '^refs/remotes/' | sed -e 's:^refs/remotes/::'
  }

  if [[ "$curr_word" == -* ]]; then
    # An option.
    case "$subcmd" in
      backout)
        COMPREPLY=( $(compgen -W '-e -edit --edit -n -no-commit --no-commit -r' -- "$curr_word") )
        return 0
        ;;
      branch)
        COMPREPLY=( $(compgen -W '-d -delete --delete -f -force --force -r' -- "$curr_word") )
        return 0
        ;;
      clone)
        COMPREPLY=( $(compgen -W '-b -branch --branch -gerrit --gerrit -gerrit-hook-url --gerrit-hook-url' -- "$curr_word") )
        return 0
        ;;
      ci|commit)
        COMPREPLY=( $(compgen -W '-amend --amend -m' -- "$curr_word") )
        return 0
        ;;
      diff)
        COMPREPLY=( $(compgen -W '-b -ignore-space-change --ignore-space-change -B -ignore-blank-lines --ignore-blank-lines -c -U -r -stat --stat -w -ignore-all-space --ignore-all-space -Z -ignore-space-at-eol --ignore-space-at-eol -M -C -copies-unmodified --copies-unmodified' -- "$curr_word") )
        return 0
        ;;
      evolve)
        COMPREPLY=( $(compgen -W '-d -dst --dst -l -list --list' -- "$curr_word") )
        return 0
        ;;
      gerrithook)
        COMPREPLY=( $(compgen -W '-url --url -cached --cached' -- "$curr_word") )
        return 0
        ;;
      histedit)
        COMPREPLY=( $(compgen -W '-abort --abort -continue --continue -edit-plan --edit-plan -exec --exec' -- "$curr_word") )
        return 0
        ;;
      id|identify)
        COMPREPLY=( $(compgen -W '-r' -- "$curr_word") )
        return 0
        ;;
      log|history)
        COMPREPLY=( $(compgen -W '-follow --follow -follow-first --follow-first -G -graph --graph -r -reverse --reverse -stat --stat' -- "$curr_word") )
        return 0
        ;;
      mail)
        COMPREPLY=( $(compgen -W '-allow-dirty --allow-dirty -d -dest --dest -for --for -r -R -reviewer --reviewer -CC --CC -cc --cc -notify --notify -notify-to --notify-to -notify-cc --notify-cc -notify-bcc --notify-bcc -m -p -publish-comments --publish-comments' -- "$curr_word") )
        return 0
        ;;
      merge)
        COMPREPLY=( $(compgen -W '-r -abort --abort' -- "$curr_word") )
        return 0
        ;;
      pull)
        COMPREPLY=( $(compgen -W '-r -tags --tags -u' -- "$curr_word") )
        return 0
        ;;
      push)
        COMPREPLY=( $(compgen -W '-f -force --force -new-branch --new-branch -r' -- "$curr_word") )
        return 0
        ;;
      rebase)
        COMPREPLY=( $(compgen -W '-base --base -dst --dst -src --src -abort --abort -continue --continue' -- "$curr_word") )
        return 0
        ;;
      remove|rm)
        COMPREPLY=( $(compgen -W '-after --after -f -force --force -r' -- "$curr_word") )
        return 0
        ;;
      requestpull|pr)
        COMPREPLY=( $(compgen -W '-body --body -draft --draft -e -edit --edit -n -dry-run --dry-run -maintainer-edits --maintainer-edits -R -reviewer --reviewer -title --title' -- "$curr_word") )
        return 0
        ;;
      revert)
        COMPREPLY=( $(compgen -W '-all --all -C -no-backup --no-backup -r' -- "$curr_word") )
        return 0
        ;;
      update|checkout|co|up)
        COMPREPLY=( $(compgen -W '-r -clean --clean -C' -- "$curr_word") )
        return 0
        ;;
      upstream)
        COMPREPLY=( $(compgen -W '-b' -- "$curr_word") )
        return 0
        ;;
      *)
        COMPREPLY=()
        return 0
        ;;
    esac
  else
    # A positional argument.
    case "$subcmd" in
      add|check|clone|evolve|init|remove|rm|st|status)
        # Commands that only deal with files.
        compopt -o nospace -o filenames
        COMPREPLY=( $(compgen -f -- "$curr_word") )
        return 0
        ;;
      backout|branch|checkout|co|histedit|id|identify|merge|rebase|up|update|upstream)
        # Commands that only deal with revisions.
        COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
        return 0
        ;;
      ci|commit)
        case "$prev_word" in
          -m)
            # Don't complete for message.
            COMPREPLY=()
            return 0
            ;;
          *)
            compopt -o nospace -o filenames
            COMPREPLY=( $(compgen -f -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
      diff)
        case "$prev_word" in
          -c|-r)
            COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
            return 0
            ;;
          *)
            compopt -o nospace -o filenames
            COMPREPLY=( $(compgen -f -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
      gerrithook)
        COMPREPLY=( $(compgen -W 'on off' -- "$curr_word") )
        return 0
        ;;
      log|history)
        case "$prev_word" in
          -r)
            COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
            return 0
            ;;
          *)
            compopt -o nospace -o filenames
            COMPREPLY=( $(compgen -f -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
      mail)
        case "$prev_word" in
          -r|-d|-dest|--dest|-for|--for)
            COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
            return 0
            ;;
          -m|-R|-reviewer|--reviewer|-CC|--CC|-cc|--cc|-notify-to|--notify-to|-notify-cc|--notify-cc|-notify-bcc|--notify-bcc)
            # Don't complete for message or emails.
            COMPREPLY=()
            return 0
            ;;
          -notify|--notify)
            COMPREPLY=( $(compgen -W 'none owner owner_reviewers all' -- "$curr_word") )
            return 0
            ;;
          *)
            COMPREPLY=( $(compgen -W "$(git remote)" -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
      pull)
        COMPREPLY=( $(compgen -W "$(git remote)" -- "$curr_word") )
        return 0
        ;;
      push)
        case "$prev_word" in
          -r)
            COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
            return 0
            ;;
          *)
            COMPREPLY=( $(compgen -W "$(git remote)" -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
      requestpull|pr)
        COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
        return 0
        ;;
      revert)
        case "$prev_word" in
          -r)
            COMPREPLY=( $(compgen -W "$(named_revs)" -- "$curr_word") )
            return 0
            ;;
          *)
            compopt -o nospace -o filenames
            COMPREPLY=( $(compgen -f -- "$curr_word") )
            return 0
            ;;
        esac
        ;;
    esac
  fi
  # Fallback: files.
  compopt -o nospace -o filenames
  COMPREPLY=( $(compgen -f -- "$curr_word") )
  return 0
}

complete -F _gg_complete gg
