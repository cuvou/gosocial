package notification_test

import (
	"reflect"
	"testing"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/notification"
)

func TestCompress(t *testing.T) {
	type testcase struct {
		Input  []*notification.Notification
		Expect []*notification.Notification
	}

	var (
		Alice = &models.User{
			ID:       1,
			Username: "alice",
		}
		Bob = &models.User{
			ID:       2,
			Username: "bob",
		}
		Charles = &models.User{
			ID:       3,
			Username: "charles",
		}
		David = &models.User{
			ID:       4,
			Username: "david",
		}
	)

	var tests = []testcase{
		{
			Input: []*notification.Notification{
				// Photo #1: liked by Alice and Bob.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        100,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:       []uint64{100},
					AboutUser: Alice,
				},
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        101,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:       []uint64{101},
					AboutUser: Bob,
				},

				// An unrelated Comment notification.
				{
					Type: models.NotificationComment,
					Models: []*models.Notification{
						{
							ID:        200,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:       []uint64{200},
					AboutUser: Charles,
				},

				// Photo #2: liked by Charles.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        102,
							TableName: "photos",
							TableID:   2,
						},
					},
					IDs:       []uint64{102},
					AboutUser: Charles,
				},

				// Photo #3: liked by David only.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        103,
							TableName: "photos",
							TableID:   3,
						},
					},
					IDs:       []uint64{103},
					AboutUser: David,
				},

				// Photo #4: a second photo liked by Charles.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        104,
							TableName: "photos",
							TableID:   4,
						},
					},
					IDs:       []uint64{104},
					AboutUser: Charles,
				},

				// Charles also adds a like to Photo #1
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        105,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:       []uint64{105},
					AboutUser: Charles,
				},
			},
			Expect: []*notification.Notification{
				// Photo #1: liked by Alice and Bob, compressed.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        100,
							TableName: "photos",
							TableID:   1,
						},
						{
							ID:        101,
							TableName: "photos",
							TableID:   1,
						},
						{
							ID:        105,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:            []uint64{100, 101, 105},
					AboutUser:      Alice,
					OtherUsernames: []string{Bob.Username, Charles.Username},
					OtherUnread:    2,
				},

				// An unrelated Comment notification.
				{
					Type: models.NotificationComment,
					Models: []*models.Notification{
						{
							ID:        200,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:       []uint64{200},
					AboutUser: Charles,
				},

				// Charles liked photo #2 and 1 other (#4).
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        102,
							TableName: "photos",
							TableID:   2,
						},
						{
							ID:        104,
							TableName: "photos",
							TableID:   4,
						},
						{
							ID:        105,
							TableName: "photos",
							TableID:   1,
						},
					},
					IDs:         []uint64{102, 104, 105},
					AboutUser:   Charles,
					OtherUnread: 2,
					OtherCount:  2,
				},

				// Photo #3: liked by David only.
				{
					Type: models.NotificationLike,
					Models: []*models.Notification{
						{
							ID:        103,
							TableName: "photos",
							TableID:   3,
						},
					},
					IDs:       []uint64{103},
					AboutUser: David,
				},
			},
		},
	}

	for i, test := range tests {
		actual := notification.Compress(test.Input, 0)
		if len(actual) != len(test.Expect) {
			t.Errorf("Test #%d: expected %d rows but got %d", i, len(test.Expect), len(actual))
			continue
		}

		for j, row := range actual {
			if !reflect.DeepEqual(row, test.Expect[j]) {
				t.Errorf("Test #%d row %d: expected %+v but got %+v", i, j, test.Expect[j], row)
			}
		}
	}
}
