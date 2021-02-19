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

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/hack/lib/init.sh"

ARKTOS_CHERRYPICK_LINE_GO="// File modified by cherrypick from kubernetes on $(date +'%m/%d/%Y')"
ARKTOS_CHERRYPICK_LINE_OTHER="# File modified by cherrypick from kubernetes on $(date +'%m/%d/%Y')"
COPYRIGHT_MATCH="limitations under the License"
ARKTOS_CHERRYPICK_MATCH="File modified by cherrypick from kubernetes"
ARKTOS_CHERRYPICK_MATCH_GO="package"
ARKTOS_CHERRYPICK_MATCH_OTHER=$COPYRIGHT_MATCH
ARKTOS_COPYRIGHT_MATCH="Authors of Arktos"

REPODIRNAME="${REPODIRNAME:-$KUBE_ROOT}"
LOGDIR=${LOGDIR:-${REPODIRNAME}/_output}
LOGFILENAME=${LOGFILENAME:-"arktoscherrypick.log"}
LOGFILE=${LOGFILE:-${LOGDIR}/$LOGFILENAME}
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

if [ ! -z ${2:-} ]
then
    LOGDIR=${2}
    LOGFILE=$LOGDIR/$LOGFILENAME
fi

if [ ! -z ${1:-} ]
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
            echo "WARN: Skipping cherrypick check for in-container build as git repo is not available"
            echo "WARN: Skipping cherrypick check for in-container build as git repo is not available" >> $LOGFILE
            exit 0
        else
            echo "Running cherrypick check for repo: $REPODIRNAME, logging to $LOGFILE"
        fi
    fi
fi

get_added_files_list() {
    pushd $REPODIRNAME
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    local MERGED_COMMIT=$( git log --show-signature --oneline | grep "gpg: Signature made" | head -n 1 | cut -c1-7 )
    echo "MERGED_COMMIT: $MERGED_COMMIT, HEAD_COMMIT: $HEAD_COMMIT"
    git diff --name-only --diff-filter=A $MERGED_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "perf-tests\/clusterloader2\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/metrics\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/version" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS|arktos_copyright" >> $LOGDIR/added_files || true
    popd
}

get_modified_files_list() {
    pushd $REPODIRNAME
    local MERGED_COMMIT=$( git log --show-signature --oneline | grep "gpg: Signature made" | head -n 1 | cut -c1-7 )
    local HEAD_COMMIT=`git rev-list HEAD | head -n 1`
    echo "MERGED_COMMIT: $MERGED_COMMIT, HEAD_COMMIT: $HEAD_COMMIT"
    git diff --name-only --diff-filter=M $MERGED_COMMIT $HEAD_COMMIT | \
        egrep -v "\.git|\.md|\.bazelrc|\.json|\.pb|\.yaml|BUILD|boilerplate|vendor\/" | \
        egrep -v "perf-tests\/clusterloader2\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/metrics\/" | \
        egrep -v "staging\/src\/k8s.io\/component-base\/version" | \
        egrep -v "\.mod|\.sum|\.png|\.PNG|OWNERS" >> $LOGDIR/changed_files || true
    popd
}

check_and_add_arktos_cherrypick() {
    local REPOFILE=$1
    set +e
    cat $REPOFILE | grep "$COPYRIGHT_MATCH" > /dev/null 2>&1
    if [ $? -eq 0 ]
    then
        cat $REPOFILE | grep "$ARKTOS_CHERRYPICK_MATCH" > /dev/null 2>&1
        if [ $? -eq 0 ]
        then
            echo "WARN: File $REPOFILE already has cherry-pick comments. Ignoring." >> $LOGFILE
        else
            cat $REPOFILE | grep "$ARKTOS_COPYRIGHT_MATCH" > /dev/null 2>&1
            if [ $? -eq 0 ]
            then
                echo "WARN: File $REPOFILE already has Arktos copyright. Ignoring." >> $LOGFILE
            else
                local tstamp=$($STAT_CMD $REPOFILE)
                if [[ $REPOFILE = *.go ]] || [[ $REPOFILE = *.proto ]]
                then
                    $SED_CMD -i "/^$ARKTOS_CHERRYPICK_MATCH_GO/i $ARKTOS_CHERRYPICK_LINE_GO" $REPOFILE
                else
cat << EOF | $SED_CMD -i "/$ARKTOS_CHERRYPICK_MATCH_OTHER/r /dev/stdin" $REPOFILE

$ARKTOS_CHERRYPICK_LINE_OTHER
EOF
                fi
                echo "WARN: File $REPOFILE does not have cherry-pick comments. Adding." >> $LOGFILE
                $TOUCH_CMD "$tstamp" $REPOFILE
            fi
        fi
    else    
        echo "ERROR: File $REPOFILE does not have either K8s or Arktos copyright." >> $LOGFILE
    fi
    set -e
}

add_arktos_cherrypick() {
    echo "Inspecting cherrypick files, writing logs to $LOGFILE"
    local ADDED_FILELIST=$1
    local CHANGED_FILELIST=$2
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_add_arktos_cherrypick $REPODIRNAME/$line
        fi
    done < $CHANGED_FILELIST
    while IFS= read -r line
    do
        if [ ! -z $line ]
        then
            check_and_add_arktos_cherrypick $REPODIRNAME/$line
        fi
    done < $ADDED_FILELIST
    echo "Done."
}

if [ -d $LOGDIR ]; then
    # the directory exists
    if [[ -f "$LOGDIR/added_files" ]]; then
        rm $LOGDIR/added_files
    fi
    if [[ -f "$LOGDIR/changed_files" ]]; then
        rm $LOGDIR/changed_files
    fi
    if [[ -f "$LOGFILE" ]]; then
        rm $LOGFILE
    fi
else 
    mkdir -p $LOGDIR
fi

get_added_files_list
get_modified_files_list

add_arktos_cherrypick $LOGDIR/added_files $LOGDIR/changed_files

exit $EXIT_ERROR
