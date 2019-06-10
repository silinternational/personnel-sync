package personnel_sync

import (
	"reflect"
	"testing"
	"time"
)

func TestGenerateChangeSet(t *testing.T) {
	type args struct {
		sourcePeople      []Person
		destinationPeople []Person
	}
	tests := []struct {
		name string
		args args
		want ChangeSet
	}{
		{
			name: "creates two, deletes one, updates one",
			want: ChangeSet{
				Create: []Person{
					{
						CompareValue: "1",
					},
					{
						CompareValue: "2",
					},
				},
				Delete: []Person{
					{
						CompareValue: "3",
					},
				},
				Update: []Person{
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "case sensitive",
						},
					},
				},
			},
			args: args{
				sourcePeople: []Person{
					{
						CompareValue: "1",
					},
					{
						CompareValue: "2",
					},
					{
						CompareValue: "4",
						Attributes: map[string]string{
							"school": "harvard",
						},
					},
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "case sensitive",
						},
					},
					{
						CompareValue: "6",
						DisableChanges: true,
					},
				},
				destinationPeople: []Person{
					{
						CompareValue: "3",
					},
					{
						CompareValue: "4",
						Attributes: map[string]string{
							"school": "HARVARD",
						},
					},
					{
						CompareValue: "5",
						Attributes: map[string]string{
							"name": "CASE SENSITIVE",
						},
					},
				},
			},
		},
	}

	attrMaps := []AttributeMap{
		{
			Source:        "name",
			Destination:   "name",
			CaseSensitive: true,
		},
		{
			Source:        "school",
			Destination:   "school",
			CaseSensitive: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateChangeSet(tt.args.sourcePeople, tt.args.destinationPeople, attrMaps); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateChangeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func DoNothing() {}

func TestBatchTimer(t *testing.T) {

	type testData struct {
		batchSize int
		secondsPerBatch int
		numberOfCalls int
		expectedDelayInSeconds int
	}

	testRuns := []testData{
		{
			batchSize: 1,
			secondsPerBatch: 1,
			numberOfCalls: 2,
			expectedDelayInSeconds: 1,
		},
		{
			batchSize: 5,
			secondsPerBatch: 1,
			numberOfCalls: 5,
			expectedDelayInSeconds: 0,
		},
	}

	for _, testRun := range testRuns {
		bTimer := BatchTimer{}
		bTimer.Init(testRun.batchSize, testRun.secondsPerBatch)
		startTime := time.Now()

		for i := 0; i < testRun.numberOfCalls; i++ {
			DoNothing()
			bTimer.WaitOnBatch()
		}

		elapsedTime := time.Since(startTime)

		results := int(elapsedTime.Seconds())
		expected := testRun.expectedDelayInSeconds

		if results != expected {
			t.Errorf("BatchTimer should have taken %v second(s) to complete. Instead, it took %v seconds", expected, results)
		}
	}
}