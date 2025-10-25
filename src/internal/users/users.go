package users

import (
	"log"
	"threadfin/internal/authentication"
	"threadfin/internal/structs"
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

// Benutzerdaten speichern (WebUI)
func SaveUserData(request structs.RequestStruct) (err error) {

	var userData = request.UserData

	var newCredentials = func(userID string, newUserData map[string]interface{}) (err error) {

		var newUsername, newPassword string
		if username, ok := newUserData["username"].(string); ok {
			newUsername = username
		}

		if password, ok := newUserData["password"].(string); ok {
			newPassword = password
		}

		if len(newUsername) > 0 {
			err = authentication.ChangeCredentials(userID, newUsername, newPassword)
		}

		return
	}

	for userID, newUserData := range userData {

		err = newCredentials(userID, newUserData.(map[string]interface{}))
		if err != nil {
			return
		}

		if request.DeleteUser {
			err = authentication.RemoveUser(userID)
			return
		}

		delete(newUserData.(map[string]interface{}), "password")
		delete(newUserData.(map[string]interface{}), "confirm")

		if _, ok := newUserData.(map[string]interface{})["delete"]; ok {

			err = authentication.RemoveUser(userID)
			if err != nil {
				log.Println("failed to remove user: ", err)
				return
			}

		} else {

			err = authentication.WriteUserData(userID, newUserData.(map[string]interface{}))
			if err != nil {
				return
			}

		}

	}

	return
}
