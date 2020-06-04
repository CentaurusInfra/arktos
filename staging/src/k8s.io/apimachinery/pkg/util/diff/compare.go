/*
Copyright 2020 Authors of Arktos.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package diff

import "time"

var allowedNTPDiffInMilliSecond = uint64(5000)

// For backwards compatible - can still use ETCD revision # starting from 1 and increases by 1 as backend storage
var v2MinRevision = getRevisionNumber(time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC).UnixNano(), 0, 1)

// RevisionIsNewer is used in event comparison to check whether revision 1 is newer than
// revision 2 and should be sent/accepted/processed
func RevisionIsNewer(revision1, revision2 uint64) bool {
	if revision1 < v2MinRevision || revision2 < v2MinRevision {
		return revision1 > revision2
	}

	result, isSameCluster := revisionCompare(revision1, revision2)
	if isSameCluster {
		return result > 0
	}
	// if from different cluster, allow sent result == 0
	return result >= 0
}

// revisionCompare compares two revision #s.
// Return -1 if revision 1 is less than revision 2
// Return 0 if revision 1 is equal to revision 2
// Return 1 if revision 1 is larger than revision 2
// Note: the definition of less than, equal to, and larger than is defined as follows:
// If they are from same cluster id, their value will be directly compared
// If they are not from same cluster id, we use allowedNTPDiffInSecond to check whether
//     the two revisions happened roughly in the same time
func revisionCompare(revision1, revision2 uint64) (result int, isSameCluster bool) {
	clusterId1 := extractClusterId(revision1)
	clusterId2 := extractClusterId(revision2)
	isSameCluster = clusterId1 == clusterId2
	if isSameCluster {
		if revision1 < revision2 {
			result = -1
		} else if revision1 == revision2 {
			result = 0
		} else {
			result = 1
		}
	} else {
		ms1 := extractMilliSecond(revision1)
		ms2 := extractMilliSecond(revision2)
		if ms1 == ms2 {
			result = 0
		} else {
			var absDiff uint64

			if ms1 < ms2 {
				result = -1
				absDiff = ms2 - ms1
			} else { // ms1 > ms2
				result = 1
				absDiff = ms1 - ms2
			}

			// add NTP variation tolerance
			if absDiff < allowedNTPDiffInMilliSecond {
				result = 0
			}
		}
	}

	return
}

// Based on design, int64 (ETCD revision type) has bits
// 0-12 as event number
// 13-18 as cluster id
// TODO: Need to test whether it is the same bit number when etcd revision # is used in API server
// as we are using uint64 here. Not sure how conversion happened
func extractClusterId(rev uint64) uint64 {
	return (rev >> 13) % 64
}

func extractMilliSecond(rev uint64) uint64 {
	return rev >> 19
}

func getRevisionNumber(testTimeInNano int64, clusterId int, seq int) (rev uint64) {
	// got time stamp to milliseconds
	revision := testTimeInNano / 1000000
	// bit 13-18 is clusterId
	revision = revision<<6 + int64(clusterId)
	// bit 0-12 is sequence id
	revision = revision<<13 + int64(seq)

	return uint64(revision)
}
