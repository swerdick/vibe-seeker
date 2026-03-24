package vibes

import "time"

// RecencyWeight returns a weight based on how recently a show occurred.
//
//	Last 3 months:  1.0
//	3-6 months:     0.7
//	6-12 months:    0.4
//	12+ months:     0.2
func RecencyWeight(showDate time.Time) float32 {
	age := time.Since(showDate)
	switch {
	case age <= 3*30*24*time.Hour:
		return 1.0
	case age <= 6*30*24*time.Hour:
		return 0.7
	case age <= 12*30*24*time.Hour:
		return 0.4
	default:
		return 0.2
	}
}
