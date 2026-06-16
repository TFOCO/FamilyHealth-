package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUserStreak_IncrementAndReset(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Kolkata") // IST
	assert.NoError(t, err)

	streak := &UserStreak{}

	// 1. First time logging
	logTime1 := time.Date(2026, 6, 16, 10, 0, 0, 0, loc) // June 16, 10:00 AM IST
	inc := streak.IncrementStreak(logTime1, loc)
	assert.True(t, inc)
	assert.Equal(t, 1, streak.CurrentStreak)
	assert.Equal(t, 1, streak.MaxStreak)
	assert.Equal(t, logTime1, streak.LastLoggedDate)

	// 2. Logging again on the same local day
	logTime2 := time.Date(2026, 6, 16, 15, 0, 0, 0, loc) // June 16, 3:00 PM IST
	inc = streak.IncrementStreak(logTime2, loc)
	assert.False(t, inc)
	assert.Equal(t, 1, streak.CurrentStreak)
	assert.Equal(t, 1, streak.MaxStreak)
	assert.Equal(t, logTime2, streak.LastLoggedDate) // Should update to more recent log time

	// 3. Logging on consecutive day
	logTime3 := time.Date(2026, 6, 17, 9, 0, 0, 0, loc) // June 17, 9:00 AM IST
	inc = streak.IncrementStreak(logTime3, loc)
	assert.True(t, inc)
	assert.Equal(t, 2, streak.CurrentStreak)
	assert.Equal(t, 2, streak.MaxStreak)
	assert.Equal(t, logTime3, streak.LastLoggedDate)

	// 4. Logging with a gap (streak broken)
	logTime4 := time.Date(2026, 6, 19, 9, 0, 0, 0, loc) // June 19, 9:00 AM IST (June 18 was missed)
	inc = streak.IncrementStreak(logTime4, loc)
	assert.True(t, inc)
	assert.Equal(t, 1, streak.CurrentStreak)
	assert.Equal(t, 2, streak.MaxStreak) // MaxStreak stays 2
	assert.Equal(t, logTime4, streak.LastLoggedDate)

	// 5. Retroactive log (older date than LastLoggedDate)
	logTimeRetro := time.Date(2026, 6, 18, 9, 0, 0, 0, loc) // June 18
	inc = streak.IncrementStreak(logTimeRetro, loc)
	assert.False(t, inc)
	assert.Equal(t, 1, streak.CurrentStreak)
	assert.Equal(t, 2, streak.MaxStreak)
	assert.Equal(t, logTime4, streak.LastLoggedDate) // LastLoggedDate remains June 19

	// 6. Reset streak
	streak.ResetStreak()
	assert.Equal(t, 0, streak.CurrentStreak)
	assert.Equal(t, 2, streak.MaxStreak) // Max streak remains
}
