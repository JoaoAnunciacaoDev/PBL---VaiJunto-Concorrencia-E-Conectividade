package models

import "github.com/google/uuid"

type ReservedSegment struct {
	RideID    uuid.UUID `json:"ride_id"`
	SegmentID uuid.UUID `json:"segment_id"`
}

func (s ReservedSegment) IsValid() bool {
	return s.RideID != uuid.Nil && s.SegmentID != uuid.Nil
}
