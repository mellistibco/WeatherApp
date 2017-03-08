package activity

import (
	"sync"
)

var (
	activitiesMu sync.Mutex
	activities   = make(map[string]Activity)
)

// Register registers the specified activity
func Register(activity Activity) {
	activitiesMu.Lock()
	defer activitiesMu.Unlock()

	if activity == nil {
		panic("activity.Register: activity is nil")
	}

	id := activity.Metadata().ID

	if _, dup := activities[id]; dup {
		panic("activity.Register: activity already registered " + id)
	}

	// copy on write to avoid synchronization on access
	newActivities := make(map[string]Activity, len(activities))

	for k, v := range activities {
		newActivities[k] = v
	}

	newActivities[id] = activity
	activities = newActivities
}

// Activities gets all the registered activities
func Activities() []Activity {

	var curActivities = activities

	list := make([]Activity, 0, len(curActivities))

	for _, value := range curActivities {
		list = append(list, value)
	}

	return list
}

// Get gets specified activity
func Get(id string) Activity {
	//var curActivities = activities
	return activities[id]
}
