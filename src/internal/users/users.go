package users

import (
	"threadfin/src/internal/authentication"
	"threadfin/src/internal/structs"
)

// Neuen Benutzer anlegen (WebUI)
func SaveNewUser(request structs.RequestStruct) (err error) {

	var data = request.UserData
	var username = data["username"].(string)
	var password = data["password"].(string)

	delete(data, "password")
	delete(data, "confirm")

	userID, err := authentication.CreateNewUser(username, password)
	if err != nil {
		return
	}

	err = authentication.WriteUserData(userID, data)
	return
}
