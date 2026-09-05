package protocol

type Action string

const (
	ActionRegisterUser       Action = "register_user"
	ActionLogin              Action = "login"
	ActionGetMyProfile       Action = "get_my_profile"
	ActionLogout             Action = "logout"

	ActionRegisterVehicle    Action = "register_vehicle"
	ActionGetMyVehicle       Action = "get_my_vehicle"
	ActionUpdateVehicle      Action = "update_vehicle"
	ActionRemoveVehicle      Action = "remove_vehicle"

	ActionCreateRide         Action = "create_ride"
	ActionListMyRides        Action = "list_my_rides"
	ActionCancelRide         Action = "cancel_ride"
	ActionGetRidePassengers  Action = "get_ride_passengers"

	ActionSearchItineraries  Action = "search_itineraries"
	ActionConfirmReservation Action = "confirm_reservation"
	ActionListMyReservations Action = "list_my_reservations"
	ActionCancelReservation  Action = "cancel_reservation"
)