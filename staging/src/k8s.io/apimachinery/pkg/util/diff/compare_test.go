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

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/rand"
	"math"
	"testing"
	"time"
)

func TestExtractClusterId(t *testing.T) {
	for i := 0; i < 64; i++ {
		rev := getRevisionNumber(time.Now().UnixNano(), i, 1)
		extractedClusterId := extractClusterId(rev)
		assert.Equal(t, uint64(i), extractedClusterId, "Expecting cluster id %v but got %v", i, extractedClusterId)
	}
}

func TestExtractedMillisecond(t *testing.T) {
	for i := 0; i < 1000; i++ {
		testTimeInMS := time.Now().Unix()*int64(1000) + int64(i)
		testTimeInNano := testTimeInMS * 1000000
		rev := getRevisionNumber(testTimeInNano, 1, 1)
		extractedMS := extractMilliSecond(rev)
		assert.Equal(t, uint64(testTimeInMS), extractedMS, "Expecting time %v but got %v", testTimeInMS, extractedMS)
	}
}

func TestRevisionCompare(t *testing.T) {
	originalAllowedNTPDiff := allowedNTPDiffInMilliSecond
	testRevisionCompare(t, 0)
	testRevisionCompare(t, 500)
	testRevisionCompare(t, 1000)
	testRevisionCompare(t, 10000)
	allowedNTPDiffInMilliSecond = originalAllowedNTPDiff
}

func testRevisionCompare(t *testing.T, timeDiffInMS int64) {
	allowedNTPDiffInMilliSecond = uint64(timeDiffInMS)

	for i := 0; i < 64; i++ {
		seq1 := 10
		testTimeInNano := time.Now().UnixNano()

		for j := 0; j < 64; j++ {
			rev1 := getRevisionNumber(testTimeInNano, i, seq1)

			// seq 1 < seq 2
			seq2 := seq1 + 1
			testTimeInNano2 := testTimeInNano + (timeDiffInMS+1)*1000000
			rev2 := getRevisionNumber(testTimeInNano2, j, seq2)
			result, isSameCluster := revisionCompare(rev1, rev2)
			assertRevCompare(t, -1, result, i, j, rev1, rev2, testTimeInNano, testTimeInNano2)
			logCluster(t, i, j, rev1, rev2, isSameCluster)

			// seq 1 == seq 2
			seq2 = seq1
			if allowedNTPDiffInMilliSecond > 0 && i != j {
				testTimeInNano2 = testTimeInNano + (timeDiffInMS-1)*1000000
			} else {
				testTimeInNano2 = testTimeInNano
			}
			rev2 = getRevisionNumber(testTimeInNano2, j, seq2)
			result, isSameCluster = revisionCompare(rev1, rev2)
			assertRevCompare(t, 0, result, i, j, rev1, rev2, testTimeInNano, testTimeInNano2)
			logCluster(t, i, j, rev1, rev2, isSameCluster)

			// seq 1 > seq 2
			seq1 = seq2 + 1
			rev2 = getRevisionNumber(testTimeInNano, j, seq2)
			testTimeInNano1 := testTimeInNano + (timeDiffInMS+2)*1000000
			rev1 = getRevisionNumber(testTimeInNano1, i, seq1)
			result, isSameCluster = revisionCompare(rev1, rev2)
			assertRevCompare(t, 1, result, i, j, rev1, rev2, testTimeInNano1, testTimeInNano)
			logCluster(t, i, j, rev1, rev2, isSameCluster)
		}
	}
}

func assertRevCompare(t *testing.T, expectedResult, result interface{}, i, j int, rev1, rev2 uint64, testTime1, testTime2 int64) {
	assert.Equal(t, expectedResult, result,
		"Expecting result %v but got %v. i=%d, j=%d, rev1=%v, rev2=%v, allowedNTPDiffInMilliSecond=%v",
		expectedResult, result, i, j, rev1, rev2, allowedNTPDiffInMilliSecond)
	if expectedResult != result {
		ms1 := extractMilliSecond(rev1)
		ms2 := extractMilliSecond(rev2)
		t.Logf("Expected result %v but got %v, i=%d, j=%d, ms1=%v, ms2=%v, nano1=%v, nano2=%v, msDiff=%v, nanoDiff=%v, rev1=%v, rev2=%v, allowedNTPDiffInMilliSecond=%v",
			expectedResult, result, i, j, ms1, ms2, testTime1, testTime2, int64(ms1)-int64(ms2), testTime1-testTime2, rev1, rev2, allowedNTPDiffInMilliSecond)
	}
}

func logCluster(t *testing.T, i, j int, rev1, rev2 uint64, isSameCluster bool) {
	assert.Equal(t, i == j, isSameCluster,
		"Expecting same cluster evaluation %v but got %v. i=%d, j=%d, rev1=%v, rev2=%v, allowedNTPDiffInMilliSecond=%v",
		i == j, isSameCluster, i, j, rev1, rev2, allowedNTPDiffInMilliSecond)
	if i != j && isSameCluster {
		clusterId1 := extractClusterId(rev1)
		clusterId2 := extractClusterId(rev2)
		t.Logf("Expected same cluster %v evaluation but got %v. i=%d, j=%d, cluster1=%v, cluster2=%v, rev1 %v, rev2 %v, allowedNTPDiffInMilliSecond %v",
			i == j, isSameCluster, i, j, clusterId1, clusterId2, rev1, rev2, allowedNTPDiffInMilliSecond)
	}
}

func TestRevisionIsNewerBackwardCompatible(t *testing.T) {
	rev1 := v2MinRevision - 1
	rev2 := uint64(rand.Int63nRange(0, int64(rev1)))
	result := RevisionIsNewer(rev1, rev2)
	assert.True(t, result, "Expecting revision %v is newer than %v but not true", rev1, rev2)

	result = RevisionIsNewer(rev2, rev1)
	assert.False(t, result, "Expecting revision %v is not newer than %v but not true", rev2, rev1)

	result = RevisionIsNewer(rev1, rev1)
	assert.False(t, result, "Expecting revision %v is not newer than %v but not true", rev1, rev1)

	rev3 := uint64(rand.Int63nRange(int64(rev1+1), math.MaxInt64))
	result = RevisionIsNewer(rev3, rev1)
	assert.True(t, result, "Expecting revision %v is newer than %v but not true", rev3, rev1)

	result = RevisionIsNewer(rev1, rev3)
	assert.False(t, result, "Expecting revision %v is not newer than %v but not true", rev1, rev3)
}

// test revision based on timestamp + clusterId + sequence #
func TestRevisionIsNewerWithTimeStamp(t *testing.T) {
	originalAllowedNTPDiff := allowedNTPDiffInMilliSecond
	testRevisionIsNewer(t, 0)
	testRevisionIsNewer(t, 500)
	testRevisionIsNewer(t, 1000)
	testRevisionIsNewer(t, 10000)
	allowedNTPDiffInMilliSecond = originalAllowedNTPDiff
}

func testRevisionIsNewer(t *testing.T, timeDiffInMS int64) {
	allowedNTPDiffInMilliSecond = uint64(timeDiffInMS)

	for i := 0; i < 64; i++ {
		seq1 := 10
		testTimeInNano := time.Now().UnixNano()

		for j := 0; j < 64; j++ {
			rev1 := getRevisionNumber(testTimeInNano, i, seq1)

			// seq 1 < seq 2
			seq2 := seq1 + 1
			testTimeInNano2 := testTimeInNano + (timeDiffInMS+1)*1000000
			rev2 := getRevisionNumber(testTimeInNano2, j, seq2)
			result := RevisionIsNewer(rev2, rev1)
			assertRevCompare(t, true, result, i, j, rev1, rev2, testTimeInNano, testTimeInNano2)

			// seq 1 == seq 2
			seq2 = seq1
			if allowedNTPDiffInMilliSecond > 0 && i != j {
				testTimeInNano2 = testTimeInNano + (timeDiffInMS-1)*1000000
			} else {
				testTimeInNano2 = testTimeInNano
			}
			rev2 = getRevisionNumber(testTimeInNano2, j, seq2)
			result = RevisionIsNewer(rev2, rev1)
			if i != j {
				assertRevCompare(t, true, result, i, j, rev1, rev2, testTimeInNano, testTimeInNano2)
			} else {
				assertRevCompare(t, testTimeInNano2 > testTimeInNano, result, i, j, rev1, rev2, testTimeInNano, testTimeInNano2)
			}

			// seq 1 > seq 2
			seq1 = seq2 + 1
			rev2 = getRevisionNumber(testTimeInNano, j, seq2)
			testTimeInNano1 := testTimeInNano + (timeDiffInMS+2)*1000000
			rev1 = getRevisionNumber(testTimeInNano1, i, seq1)
			result = RevisionIsNewer(rev2, rev1)
			assertRevCompare(t, false, result, i, j, rev1, rev2, testTimeInNano1, testTimeInNano)
		}
	}
}
