#compdef _gg gg
#
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

# zsh completion docs:
# http://zsh.sourceforge.net/Doc/Release/Completion-System.html
# https://github.com/zsh-users/zsh-completions/blob/master/zsh-completions-howto.org

_gg() {
  if (( CURRENT == 2 )); then
    _values 'gg commands' \
      'add[add the specified files on the next commit]' \
      'backout[reverse effect of an earlier commit]' \
      'branch[list or manage branches]' \
      'clone[make a copy of an existing repository]' \
      {commit,ci}'[commit the specified files or all outstanding changes]' \
      'diff[diff repository (or selected files)]' \
      'evolve[sync with Gerrit changes in upstream]' \
      'gerrithook[install or uninstall Gerrit change ID hook]' \
      'histedit[interactively edit revision history]' \
      {identify,id}'[identify the working directory or specified revision]' \
      'init[create a new repository in the given directory]' \
      {log,history}'[show revision history of entire repository or files]' \
      'mail[creates or updates a Gerrit change]' \
      'merge[merge another revision into working directory]' \
      'pull[pull changes from the specified source]' \
      'push[push changes to the specified destination]' \
      'rebase[move revision (and descendants) to a different branch]' \
      {remove,rm}'[remove the specified files on the next commit]' \
      {requestpull,pr}'[create a GitHub pull request]' \
      'revert[restore files to their checkout state]' \
      {status,st,check}'[show changed files in the working directory]' \
      {update,up,checkout,co}'[update working directory (or switch revisions)]' \
      'upstream[query or set upstream branch]'
    return
  fi
  named_revs() {
    local refs=( $(git show-ref --head 2>/dev/null | sed -e 's/^\S\+ //') )
    local all_revs=( "${refs[@]}" )
    all_revs+=( $(print -l "${refs[@]}" | grep '^refs/heads/' | sed -e 's:^refs/heads/::') )
    all_revs+=( $(print -l "${refs[@]}" | grep '^refs/tags/' | sed -e 's:^refs/tags/::') )
    all_revs+=( $(print -l "${refs[@]}" | grep '^refs/remotes/' | sed -e 's:^refs/remotes/::') )
    _wanted revisions expl 'revision' compadd -a all_revs
  }
  branches() {
    local branches=( $(git show-ref 2>/dev/null | sed -e 's/^\S\+ //' | sed -n -e 's:^refs/heads/::p') )
    _wanted branches expl 'branch' compadd -a branches
  }
  remotes() {
    local remotes=( $(git remote) )
    _wanted remotes expl 'remote' compadd -a remotes
  }
  case "${words[2]}" in
    add)
      _arguments -S : \
        ':command:' \
        '*:file:_files'
      ;;
    backout)
      _arguments -S : \
        ':command:' \
        {-e,-edit}'[invoke editor on commit message]' \
        {-n,-no-commit}'[do not commit]' \
        '-r=[revision]:rev:named_revs' \
        ':rev:named_revs'
      ;;
    branch)
      _arguments -S : \
        ':command:' \
        {-d,-delete}'[delete the given branch]' \
        {-f,-force}'[force]' \
        '-r=[revision]:rev:named_revs' \
        '*:name:branches'
      ;;
    clone)
      _arguments -S : \
        ':command:' \
        {-b,-branch}'=[branch to check out]' \
        '-gerrit[install Gerrit hook]' \
        '-gerrit-hook-url=[URL of hook script to download]' \
        ':url:' \
        ':dest:_files'
      ;;
    commit|ci)
      _arguments -S : \
        ':command:' \
        '-amend[amend the parent of the working directory]' \
        '-m=[use text as commit message]:message:' \
        '*:file:_files'
      ;;
    diff)
      _arguments -S : \
        ':command:' \
        {-b,-ignore-space-change}'[ignore changes in amount of whitespace]' \
        {-B,-ignore-blank-lines}'[ignore changes whose lines are all blank]' \
        '-c=[change made by revision]:rev:named_revs' \
        '-U=[number of lines of context to show]' \
        '*-r=[revision]:rev:named_revs' \
        '-stat[output diffstat-style summary of changes]' \
        {-w,-ignore-all-space}'[ignore whitespace when comparing lines]' \
        {-Z,-ignore-space-at-eol}'[ignore changes in whitespace at EOL]' \
        '-M=[report new files with the set percentage of similarity to a removed file as renamed]' \
        '-C=[report new files with the set percentage of similarity as copied]' \
        '-copies-unmodified[whether to check unmodified files when detecting copies (can be expensive)]' \
        '*:file:_files'
      ;;
    evolve)
      _arguments -S : \
        ':command:' \
        {-d,-dst}'[ref to compare with (defaults to upstream)]:ref:named_revs' \
        {-l,-list}'[list commits with match change IDs]'
      ;;
    gerrithook)
      _arguments -S : \
        ':command:' \
        '-url=[URL of hook script to download]' \
        '-cached[Use local cache instead of downloading]' \
        ':on/off:(on off)'
      ;;
    histedit)
      _arguments -S : \
        ':command:' \
        - start \
        '*-exec=[execute the shell command after each line creating a commit]:command:_command_names -e' \
        ':upstream:named_revs' \
        - abort \
        '-abort[abort an edit already in progress]' \
        - 'continue' \
        '-continue[continue an edit already in progress]' \
        - 'edit-plan' \
        '-edit-plan[edit remaining actions list]'
      ;;
    identify|id)
      _arguments -S : \
        ':command:' \
        '-r=[revision]:rev:named_revs'
      ;;
    init)
      _arguments -S : \
        ':command:' \
        '*:file:_files'
      ;;
    log|history)
      _arguments -S : \
        ':command:' \
        '-follow[follow file history across copies and renames]' \
        '-follow-first[only follow the first parent of merge commits]' \
        {-G,-graph}'[show the revision DAG]' \
        '*-r=[show the specified revision or range]:rev:named_revs' \
        '-reverse[reverse order of commits]' \
        '-stat[include diffstat-style summary of each commit]' \
        '*:file:_files'
      ;;
    mail)
      _arguments -S : \
        ':command:' \
        '-allow-dirty[allow mailing when working copy has uncommitted changes]' \
        {-d,-dest,-for}'=[destination branch]:branch:branches' \
        '-r=[source revision]:rev:named_revs' \
        '*'{-R,-reviewer}'=[reviewer email]:email:_email_addresses' \
        '*'{-CC,-cc}'=[emails to CC]:email:_email_addresses' \
        '-notify=[who to send email notifications to]::(none owner owner_reviewers all)' \
        '*-notify-to=[emails to send notification]:email:_email_addresses' \
        '*-notify-cc=[emails to CC notification]:email:_email_addresses' \
        '*-notify-bcc=[emails to BCC notification]:email:_email_addresses' \
        '-m=[use text as comment message]' \
        {-p,-publish-comments}'[publish draft comments]' \
        ':destination:remotes'
      ;;
    merge)
      _arguments -S : \
        ':command:' \
        - arg \
        ':rev:named_revs' \
        - rflag \
        '-r=[revision to merge]:rev:named_revs' \
        - abort \
        '-abort[abort the ongoing merge]'
      ;;
    pull)
      _arguments -S : \
        ':command:' \
        '-r=[remote reference intended to be pulled]:remote ref:branches' \
        '-tags[pull all tags from remote]' \
        '-u[update to new head if new descendants were pulled]' \
        ':source:remotes'
      ;;
    push)
      _arguments -S : \
        ':command:' \
        '-create[allow pushing a new ref]' \
        {-d,-dest}'=[destination ref]' \
        '-f[allow overwriting ref if it is not an ancestor, as long as it matches the remote-tracking branch]' \
        {-n,-dry-run}'[do everything except send the changes]' \
        '-r=[source revision]:rev:named_revs' \
        ':destination:remotes'
      ;;
    rebase)
      _arguments -S : \
        ':command:' \
        - start \
        '(-src)-base=[rebase everything from branching point of specified revision]:rev:named_revs' \
        '(-base)-src=[rebase the specified revision and descendants]:rev:named_revs' \
        '-dst=[rebase onto the specified revision]:rev:named_revs' \
        - abort \
        '-abort[abort an interrupted rebase]' \
        - 'continue' \
        '-continue[continue an interrupted rebase]' \
      ;;
    remove|rm)
      _arguments -S : \
        ':command:' \
        '-after[record delete for missing files]' \
        {-f,-force}'[forget added files, delete modified files]' \
        '-r[remove files under any directory specified]' \
        '*:file:_files'
      ;;
    requestpull|pr)
      _arguments -S : \
        ':command:' \
        '(-body -title)'{-e,-edit}'[invoke editor on pull request message]' \
        '(-e -edit)-body=[pull request description]' \
        '(-e -edit)-title=[pull request title]' \
        '-draft[create a pull request as draft]' \
        {-n,-dry-run}'[prints the pull request instead of creating it]' \
        '-maintainer-edits=[allow maintainers to edit this branch]:on/off:(0 1)' \
        '*'{-R,-reviewer}'=[GitHub usernames of reviewers to add]:user:' \
        ':branch:branches'
      ;;
    revert)
      _arguments -S : \
        ':command:' \
        {-C,-no-backup}'[do not save backup copies of files]' \
        '-r=[revert to specified revision]:rev:named_revs' \
        - all \
        '-all[revert all changes]' \
        - files \
        '*:file:_files'
      ;;
    status|check|st)
      _arguments -S : \
        ':command:' \
        '*:file:_files'
      ;;
    update|checkout|co|up)
      _arguments -S : \
        ':command:' \
        {-C,-clean}'[discard uncommitted changes (no backup)]' \
        - arg \
        ':rev:named_revs' \
        - rflag \
        '-r=[revision]:rev:named_revs'
      ;;
    upstream)
      _arguments -S : \
        ':command:' \
        '-b=[branch to query or modify]:branch:branches' \
        ':ref:named_revs'
      ;;
  esac
}
