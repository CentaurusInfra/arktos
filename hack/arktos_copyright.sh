#!/bin/bash -e

# Copyright 2020 Authors of Arktos.
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

ARKTOS_COPYRIGHT_LINE_NEW_GO="Copyright 2020 Authors of Arktos."
ARKTOS_COPYRIGHT_LINE_NEW_OTHER="# Copyright 2020 Authors of Arktos."
ARKTOS_COPYRIGHT_LINE_MODIFIED_GO="Copyright 2020 Authors of Arktos - file modified."
ARKTOS_COPYRIGHT_LINE_MODIFIED_OTHER="# Copyright 2020 Authors of Arktos - file modified."
K8S_COPYRIGHT_MATCH="The Kubernetes Authors"
ARKTOS_COPYRIGHT_MATCH="Authors of Arktos"

ARKTOS_REPO="https://github.com/futurewei-cloud/arktos"
TMPDIR="/tmp/ArktosCopyright"
HEADDIRNAME="HEAD"
REPODIRNAME=$TMPDIR/$HEADDIRNAME
LOGFILENAME="ArktosCopyrightTool.log"
LOGDIR=$TMPDIR
LOGFILE=$LOGDIR/$LOGFILENAME
EXIT_ERROR=0

SED_CMD=""
STAT_CMD=""
TOUCH_CMD=""
if [[ "$OSTYPE" == "darwin"* ]]
then
    SED_CMD=`which gsed`
    if [ -z $SED_CMD ]
    then
        echo "Please install gnu-sed (brew install gnu-sed)"
        exit 1
    fi
    STAT_CMD="stat -f %Sm -t %Y%m%d%H%M.%S "
    TOUCH_CMD="touch -mt "
elif [[ "$OSTYPE" == "linux"* ]]
then
    SED_CMD=`which sed`
    if [ -z $SED_CMD ]
    then
        echo "Please install sed"
        exit 1
    fi
    STAT_CMD="stat -c %y "
    TOUCH_CMD="touch -d "
else
    echo "Unsupported OS $OSTYPE"
    exit 1
fi

display_usage() {
    echo "Usage: $0 <optional-arktos-repo-path> <optional-log-directory>"
    echo "       If optional Arktos repo path is provided, repo setup step will be skipped"
}

if [ ! -z $2 ]
then
    LOGDIR=$2
    LOGFILE=$LOGDIR/$LOGFILENAME
fi

if [ ! -z $1 ]
then
    if [[ ( $1 == "--help") ||  $1 == "-h" ]]
    then
        display_usage
        exit 0
    else
        REPODIRNAME=$1
        if [ -z $2 ]
        then
	    LOGFILE=$REPODIRNAME/../$LOGFILENAME
        fi
        rm -f $LOGFILE
        inContainer=true
        if [[ -f /proc/1/sched ]]
        then
            PROC1=`cat /proc/1/sched | head -n 1`
            if [[ $PROC1 == systemd* ]]
            then
                inContainer=false
            fi
        else
            if [[ "$OSTYPE" == "darwin"* ]]
            then
                inContainer=false
            fi
        fi
        if [ "$inContainer" = true ]
        then
            echo "WARN: Skipping copyright check for in-container build as git repo is not available"
            echo "WARN: Skipping copyright check for in-container build as git repo is not available" >> $LOGFILE
            exit 0
        else
            echo "Running copyright check for repo: $REPODIRNAME, logging to $LOGFILE"
        fi
    fi
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
    clone_repo $ARKTOS_REPO $REPODIRNAME
}

get_added_files_list() {
    pushd $REPODIRNAME
    local DAY0_COMMIT=`git rev-list --max-parents=0 HEAD | tail -n 1`
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    git diff --name-only --diff-filter=A $DAY0_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "perf-tests\/clusterloader2\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/metrics\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/version" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS|arktos_copyright" > $LOGDIR/added_files_git
    git diff --cached --name-only --diff-filter=A | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS|arktos_copyright" >> $LOGDIR/added_files_git || true
    grep -F -x -v -f $REPODIRNAME/hack/arktos_copyright_copied_k8s_files $LOGDIR/added_files_git > $LOGDIR/added_files_less_copied
    grep -F -x -v -f $REPODIRNAME/hack/arktos_copyright_copied_modified_k8s_files $LOGDIR/added_files_less_copied > $LOGDIR/added_files
    popd
}

get_modified_files_list() {
    pushd $REPODIRNAME
    local DAY0_COMMIT=`git rev-list --max-parents=0 HEAD | tail -n 1`
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    git diff --name-only --diff-filter=M $DAY0_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "perf-tests\/clusterloader2\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/metrics\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/version" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS" > $LOGDIR/changed_files
    git diff --cached --name-only --diff-filter=M | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS|arktos_copyright" >> $LOGDIR/changed_files || true
    cat $REPODIRNAME/hack/arktos_copyright_copied_modified_k8s_files >> $LOGDIR/changed_files
    popd
}

replace_k8s_copyright_with_arktos_copyright() {
    local REPOFILE=$1
    local tstamp=$($STAT_CMD $REPOFILE)
    if [[ $REPOFILE = *.go ]] || [[ $REPOFILE = *.proto ]]
    then
        $SED_CMD -i "/$K8S_COPYRIGHT_MATCH/s/.*/$ARKTOS_COPYRIGHT_LINE_NEW_GO/" $REPOFILE
    else
        $SED_CMD -i "/$K8S_COPYRIGHT_MATCH/s/.*/$ARKTOS_COPYRIGHT_LINE_NEW_OTHER/" $REPOFILE
    fi
    $TOUCH_CMD "$tstamp" $REPOFILE
}

check_and_add_arktos_copyright() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$K8S_COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ARKTOS_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "ERROR: Added file $REPOFILE has both K8s and Arktos copyright." >> $LOGFILE
        else
            echo "WARN: Added file $REPOFILE has K8s copyright and not Arktos copyright. Replacing." >> $LOGFILE
            replace_k8s_copyright_with_arktos_copyright $REPOFILE
        fi
    else
        cat $REPOFILE | grep "$ARKTOS_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "Added file $REPOFILE has only Arktos copyright. Skipping." >> $LOGFILE
        else
            echo "ERROR: Added file $REPOFILE does not have either K8s or Arktos copyright." >> $LOGFILE
        fi
    fi
    set -e
}

update_arktos_copyright() {
    local REPOFILE=$1
    local tstamp=$($STAT_CMD $REPOFILE)
    if [[ $REPOFILE = *.go ]] || [[ $REPOFILE = *.proto ]]
    then
        $SED_CMD -i "/$K8S_COPYRIGHT_MATCH/a $ARKTOS_COPYRIGHT_LINE_MODIFIED_GO" $REPOFILE
    else
        $SED_CMD -i "/$K8S_COPYRIGHT_MATCH/a $ARKTOS_COPYRIGHT_LINE_MODIFIED_OTHER" $REPOFILE
    fi
    $TOUCH_CMD "$tstamp" $REPOFILE
}

check_and_update_arktos_copyright() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$K8S_COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ARKTOS_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "Modified file $REPOFILE has both K8s and Arktos copyright. Skipping." >> $LOGFILE
        else
            echo "Modified file $REPOFILE has K8s copyright but not Arktos copyright. Patching." >> $LOGFILE
            update_arktos_copyright $REPOFILE
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
        cat $REPOFILE | grep "$ARKTOS_COPYRIGHT_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "WARN: Copied file $REPOFILE has both K8s and Arktos copyright. Patching." >> $LOGFILE
            local tstamp=$($STAT_CMD $REPOFILE)
            $SED_CMD -i "/$ARKTOS_COPYRIGHT_MATCH/d" $REPOFILE
            $TOUCH_CMD "$tstamp" $REPOFILE
        else
            echo "Copied file $REPOFILE has K8s copyright but not Arktos copyright. Skipping." >> $LOGFILE
        fi
    else
        echo "ERROR: Copied file $REPOFILE does not have K8s copyright. Please fix manually." >> $LOGFILE
        echo "ERROR: Copied file $REPOFILE does not have K8s copyright. Please fix manually."
        EXIT_ERROR=1
    fi
    set -e
}

add_arktos_copyright() {
    echo "Inspecting copyright files, writing logs to $LOGFILE"
    local ADDED_FILELIST=$1
    local CHANGED_FILELIST=$2
    local COPIED_FILELIST=$3
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_update_arktos_copyright $REPODIRNAME/$line
        fi
    done < $CHANGED_FILELIST
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_add_arktos_copyright $REPODIRNAME/$line
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

add_arktos_copyright $LOGDIR/added_files $LOGDIR/changed_files $REPODIRNAME/hack/arktos_copyright_copied_k8s_files

exit $EXIT_ERROR
