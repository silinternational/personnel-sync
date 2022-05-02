package internal

import (
	"log"
	"os"
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
						CompareValue:   "6",
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

	config := Config{
		AttributeMap: []AttributeMap{
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := log.New(os.Stdout, "", 0)
			got := GenerateChangeSet(logger, tt.args.sourcePeople, tt.args.destinationPeople, config)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateChangeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func DoNothing() {}

func TestBatchTimer(t *testing.T) {
	type testData struct {
		batchSize              int
		secondsPerBatch        int
		numberOfCalls          int
		expectedDelayInSeconds int
	}

	testRuns := []testData{
		{
			batchSize:              1,
			secondsPerBatch:        1,
			numberOfCalls:          2,
			expectedDelayInSeconds: 1,
		},
		{
			batchSize:              5,
			secondsPerBatch:        1,
			numberOfCalls:          5,
			expectedDelayInSeconds: 0,
		},
	}

	for _, testRun := range testRuns {
		bTimer := NewBatchTimer(testRun.batchSize, testRun.secondsPerBatch)
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

func TestIDSetForUpdate(t *testing.T) {
	sourcePeople := []Person{
		{
			CompareValue: "user1@domain.com",
			Attributes: map[string]string{
				"email": "user1@domain.com",
				"name":  "before",
			},
		},
		{
			CompareValue: "user2@domain.com",
			Attributes: map[string]string{
				"email": "user2@domain.com",
				"name":  "before",
			},
		},
		{
			CompareValue: "user3@domain.com",
			Attributes: map[string]string{
				"email": "user3@domain.com",
			},
		},
	}

	destinationPeople := []Person{
		{
			CompareValue: "user1@domain.com",
			Attributes: map[string]string{
				"id":    "1",
				"email": "user1@domain.com",
				"name":  "after",
			},
		},
		{
			CompareValue: "user2@domain.com",
			Attributes: map[string]string{
				"id":    "2",
				"email": "user2@domain.com",
				"name":  "after",
			},
		},
	}

	config := Config{
		AttributeMap: []AttributeMap{
			{
				Source:        "email",
				Destination:   "email",
				Required:      true,
				CaseSensitive: false,
			},
		},
	}

	logger := log.New(os.Stdout, "", 0)
	changeSet := GenerateChangeSet(logger, sourcePeople, destinationPeople, config)
	if len(changeSet.Create) != 1 {
		t.Error("Change set should include one person to be created.")
	}
	if changeSet.Create[0].ID != "" {
		t.Error("The user to be created has an ID but shouldn't")
	}
	if len(changeSet.Update) != 2 {
		t.Error("Change set should include two people to be updated")
	}
	for _, person := range changeSet.Update {
		if person.ID == "" {
			t.Errorf("Users to be updated should have an ID set, got: %v", person)
		}
	}
}

func Test_processExpressions(t *testing.T) {
	type args struct {
		logger *log.Logger
		config Config
		person Person
	}
	logger := log.New(os.Stdout, "", 0)
	tests := []struct {
		name string
		args args
		want Person
	}{
		{
			name: "no expression",
			args: args{
				logger: logger,
				config: Config{
					AttributeMap: []AttributeMap{{
						Destination:   "first_name",
						Required:      false,
						CaseSensitive: false,
						Expression:    "",
						Replace:       "",
					}},
				},
				person: Person{
					Attributes: map[string]string{
						"first_name": "John",
					},
				},
			},
			want: Person{
				Attributes: map[string]string{
					"first_name": "John",
				},
			},
		},
		{
			name: "full replace",
			args: args{
				logger: logger,
				config: Config{
					AttributeMap: []AttributeMap{{
						Destination:   "first_name",
						Required:      false,
						CaseSensitive: false,
						Expression:    ".*",
						Replace:       "(empty)",
					}},
				},
				person: Person{
					Attributes: map[string]string{
						"first_name": "John",
					},
				},
			},
			want: Person{
				Attributes: map[string]string{
					"first_name": "(empty)",
				},
			},
		},
		{
			name: "partial replace",
			args: args{
				logger: logger,
				config: Config{
					AttributeMap: []AttributeMap{{
						Destination:   "first_name",
						Required:      false,
						CaseSensitive: false,
						Expression:    "ohn",
						Replace:       "uan",
					}},
				},
				person: Person{
					Attributes: map[string]string{
						"first_name": "John",
					},
				},
			},
			want: Person{
				Attributes: map[string]string{
					"first_name": "Juan",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processExpressions(tt.args.logger, tt.args.config, tt.args.person); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processExpressions() = %v, want %v", got, tt.want)
			}
		})
	}
}
