package models

import (
	"time"

	"github.com/google/uuid"
)

type Ride struct {
	ID          uuid.UUID `json:"id"`
	DriverID    uuid.UUID `json:"driver_id"`
	DepartureAt time.Time `json:"departure_at"`
	Segments    []Stage   `json:"segments"`
	Cancelled   bool      `json:"cancelled"`
}

func (r *Ride) IsValid() bool {
	if r.ID == uuid.Nil || r.DriverID == uuid.Nil || r.DepartureAt.IsZero() || len(r.Segments) == 0 {
		return false
	}

	return areSegmentsContinuous(r.Segments)
}

func (r *Ride) Cancel() {
	r.Cancelled = true
}

func (r *Ride) AddSegment(segment Stage) bool {
	if !segment.IsValid() {
		return false
	}

	if len(r.Segments) > 0 {
		lastSegment := r.Segments[len(r.Segments)-1]
		if lastSegment.Destination != segment.Origin {
			return false
		}
	}

	r.Segments = append(r.Segments, segment)
	return true
}

func (r *Ride) RemoveSegment(index int) bool {
	if index < 0 || index >= len(r.Segments) {
		return false
	}

	updatedSegments := append([]Stage(nil), r.Segments[:index]...)
	updatedSegments = append(updatedSegments, r.Segments[index+1:]...)

	if len(updatedSegments) == 0 || !areSegmentsContinuous(updatedSegments) {
		return false
	}

	r.Segments = updatedSegments
	return true
}

func (r *Ride) UpdateSegment(index int, segment Stage) bool {
	if index < 0 || index >= len(r.Segments) || !segment.IsValid() {
		return false
	}

	updatedSegments := append([]Stage(nil), r.Segments...)
	updatedSegments[index] = segment
	if !areSegmentsContinuous(updatedSegments) {
		return false
	}

	r.Segments = updatedSegments
	return true
}

func (r *Ride) TotalPriceCents() int {
	totalPrice := 0

	for _, segment := range r.Segments {
		totalPrice += segment.PriceCents
	}

	return totalPrice
}

func (r *Ride) MinimumAvailableSeats() int {
	if r.Cancelled || len(r.Segments) == 0 {
		return 0
	}

	minimumAvailableSeats := r.Segments[0].AvailableSeats
	for _, segment := range r.Segments[1:] {
		if segment.AvailableSeats < minimumAvailableSeats {
			minimumAvailableSeats = segment.AvailableSeats
		}
	}

	return minimumAvailableSeats
}

func areSegmentsContinuous(segments []Stage) bool {
	for index, segment := range segments {
		if !segment.IsValid() {
			return false
		}

		if index > 0 && segments[index-1].Destination != segment.Origin {
			return false
		}
	}

	return true
}
