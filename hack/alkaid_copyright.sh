#!/bin/bash -e

# Copyright 2020 Authors of Alkaid.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ALKAID_COPYRIGHT_LINE_NEW_GO="Copyright 2020 Authors of Alkaid."
ALKAID_COPYRIGHT_LINE_NEW_OTHER="# Copyright 2020 Authors of Alkaid."
ALKAID_COPYRIGHT_LINE_MODIFIED_GO="Copyright 2020 Authors of Alkaid - file modified."
ALKAID_COPYRIGHT_LINE_MODIFIED_OTHER="# Copyright 2020 Authors of Alkaid - file modified."
K8S_COPYRIGHT_MATCH="The Kubernetes Authors"
ALKAID_COPYRIGHT_MATCH="Authors of Alkaid"

ALKAID_REPO="https://github.com/futurewei-cloud/alkaid"
TMPDIR="/tmp/AlkaidCopyright"
HEADDIRNAME="HEAD"
REPODIRNAME=$TMPDIR/$HEADDIRNAME
LOGFILENAME="AlkaidCopyrightTool.log"
LOGDIR=$TMPDIR
LOGFILE=$LOGDIR/$LOGFILENAME
EXIT_ERROR=0

display_usage() {
    echo "Usage: $0 <optional-alkaid-repo-path> <optional-log-directory>"
    echo "       If optional Alkaid repo path is provided, repo setup step will be skipped"
}

if [ ! -z $1 ]
then
    if [[ ( $1 == "--help") ||  $1 == "-h" ]]
    then
        display_usage
        exit 0
    else
        echo "Running copyright check for repo: $1"
        REPODIRNAME=$1
	LOGFILE=$REPODIRNAME/../$LOGFILENAME
    fi
fi

if [ ! -z $2 ]
then
    LOGDIR=$2
    LOGFILE=$LOGDIR/$LOGFILENAME
fi

clone_repo() {
    local REPO=$1
    local DESTDIR=$2
    git clone $REPO $DESTDIR
}

setup_repos() {
    if [ -d $TMPDIR ]; then
        rm -rf $TMPDIR
    fi
    mkdir -p $TMPDIR
    clone_repo $ALKAID_REPO $REPODIRNAME
}

get_added_files_list() {
    pushd $REPODIRNAME
    local DAY0_COMMIT=`git rev-list --max-parents=0 HEAD | tail -n 1`
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    git diff --name-only --diff-filter=A $DAY0_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS|alkaid_copyright" > $LOGDIR/added_files_git
    grep -F -x -v -f $REPODIRNAME/hack/alkaid_copyright_copied_k8s_files $LOGDIR/added_files_git > $LOGDIR/added_files_less_copied
    grep -F -x -v -f $REPODIRNAME/hack/alkaid_copyright_copied_modified_k8s_files $LOGDIR/added_files_less_copied > $LOGDIR/added_files
    popd
}

get_modified_files_list() {
    pushd $REPODIRNAME
    local DAY0_COMMIT=`git rev-list --max-parents=0 HEAD | tail -n 1`
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    git diff --name-only --diff-filter=M $DAY0_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS" > $LOGDIR/changed_files
    cat $REPODIRNAME/hack/alkaid_copyright_copied_modified_k8s_files >> $LOGDIR/changed_files
    popd
}

replace_k8s_copyright_with_alkaid_copyright() {
    local REPOFILE=$1
    if [[ $REPOFILE = *.go ]]
    then
        sed -i "/$K8S_COPYRIGHT_MATCH/s/.*/$ALKAID_COPYRIGHT_LINE_NEW_GO/" $REPOFILE
    else
        sed -i "/$K8S_COPYRIGHT_MATCH/s/.*/$ALKAID_COPYRIGHT_LINE_NEW_OTHER/" $REPOFILE
    fi
}

check_and_add_alkaid_copyright() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$K8S_COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ALKAID_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "ERROR: Added file $REPOFILE has both K8s and Alkaid copyright." >> $LOGFILE
        else
            echo "WARN: Added file $REPOFILE has K8s copyright and not Alkaid copyright. Replacing." >> $LOGFILE
            replace_k8s_copyright_with_alkaid_copyright $REPOFILE
        fi
    else
        cat $REPOFILE | grep "$ALKAID_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "Added file $REPOFILE has only Alkaid copyright. Skipping." >> $LOGFILE
        else
            echo "ERROR: Added file $REPOFILE does not have either K8s or Alkaid copyright." >> $LOGFILE
        fi
    fi
    set -e
}

update_alkaid_copyright() {
    local REPOFILE=$1
    if [[ $REPOFILE = *.go ]]
    then
        sed -i "/$K8S_COPYRIGHT_MATCH/a $ALKAID_COPYRIGHT_LINE_MODIFIED_GO" $REPOFILE
    else
        sed -i "/$K8S_COPYRIGHT_MATCH/a $ALKAID_COPYRIGHT_LINE_MODIFIED_OTHER" $REPOFILE
    fi
}

check_and_update_alkaid_copyright() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$K8S_COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ALKAID_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "Modified file $REPOFILE has both K8s and Alkaid copyright. Skipping." >> $LOGFILE
        else
            echo "Modified file $REPOFILE has K8s copyright but not Alkaid copyright. Patching." >> $LOGFILE
            update_alkaid_copyright $REPOFILE
        fi
    else
        echo "Modified file $REPOFILE does not have K8s copyright. Skipping." >> $LOGFILE
    fi
    set -e
}

verify_copied_file_copyright() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$K8S_COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ALKAID_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "WARN: Copied file $REPOFILE has both K8s and Alkaid copyright. Patching." >> $LOGFILE
            sed -i "/$ALKAID_COPYRIGHT_MATCH/d" $REPOFILE
        else
            echo "Copied file $REPOFILE has K8s copyright but not Alkaid copyright. Skipping." >> $LOGFILE
        fi
    else
        echo "ERROR: Copied file $REPOFILE does not have K8s copyright. Please fix manually." >> $LOGFILE
        echo "ERROR: Copied file $REPOFILE does not have K8s copyright. Please fix manually."
        EXIT_ERROR=1
    fi
    set -e
}

add_alkaid_copyright() {
    echo "Inspecting copyright files, writing logs to $LOGFILE"
    rm -f $LOGFILE
    local ADDED_FILELIST=$1
    local CHANGED_FILELIST=$2
    local COPIED_FILELIST=$3
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_update_alkaid_copyright $REPODIRNAME/$line
        fi
    done < $CHANGED_FILELIST
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_add_alkaid_copyright $REPODIRNAME/$line
        fi
    done < $ADDED_FILELIST
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
           verify_copied_file_copyright $REPODIRNAME/$line
        fi
    done < $COPIED_FILELIST
    echo "Done."
}

if [ -z $1 ]
then
    setup_repos
fi

get_added_files_list
get_modified_files_list

add_alkaid_copyright $LOGDIR/added_files $LOGDIR/changed_files $REPODIRNAME/hack/alkaid_copyright_copied_k8s_files

exit $EXIT_ERROR
