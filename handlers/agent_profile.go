package handlers

import (
	"asira_borrower/models"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dgrijalva/jwt-go"
	"github.com/lib/pq"
	"github.com/thedevsaddam/govalidator"

	"github.com/labstack/echo"
)

type (
	AgentPayload struct {
		Email string  `json:"email"`
		Phone string  `json:"phone"`
		Banks []int64 `json:"banks"`
		Image string  `json:"image"`
	}

	BanksResponse struct {
		Name string `json:"name"`
	}

	AgentResponse struct {
		models.Agent
		BankNames pq.StringArray `json:"bank_names"`
	}
)

//AgentProfile get current agent's profile
func AgentProfile(c echo.Context) error {
	defer c.Request().Body.Close()

	user := c.Get("user")
	token := user.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)

	// agentModel := models.Agent{}

	agentID, _ := strconv.Atoi(claims["jti"].(string))

	//set banks name
	agentBank := AgentResponse{}
	db := asira.App.DB.Table("agents ag").
		Select("ag.*, (SELECT ARRAY_AGG(name) FROM banks WHERE id IN (SELECT UNNEST(ag.banks))) as bank_names").
		Where("ag.id = ?", agentID)

	err = db.Find(&agentBank).Error
	if err != nil {
		return returnInvalidResponse(http.StatusForbidden, err, "Akun tidak valid")
	}

	//co py to array string
	// banks := []string{}
	// for _, data := range banksName {
	// 	banks = append(banks, data.Name)
	// }

	//set response
	// response := AgentResponse{agentModel, banks}
	return c.JSON(http.StatusOK, agentBank)
}

//AgentProfileEdit update current agent's profile
func AgentProfileEdit(c echo.Context) error {
	defer c.Request().Body.Close()
	var agentPayload AgentPayload

	user := c.Get("user")
	token := user.(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	agentID, _ := strconv.Atoi(claims["jti"].(string))

	//cek agent with custom field (name of banks)
	agentModel := models.Agent{}
	err := agentModel.FindbyID(agentID)
	if err != nil {
		return returnInvalidResponse(http.StatusForbidden, err, "Akun tidak ditemukan")
	}

	//securing old password
	password := agentModel.Password

	payloadRules := govalidator.MapData{
		"email": []string{"email"},
		"phone": []string{"id_phonenumber"},
		"banks": []string{"valid_id:banks"},
		"image": []string{},
	}

	//validate request data
	validate := validateRequestPayload(c, payloadRules, &agentPayload)
	if validate != nil {
		return returnInvalidResponse(http.StatusUnprocessableEntity, validate, "validation error")
	}

	//cek unique for patching
	uniques := map[string]string{
		"email": agentPayload.Email,
		"phone": agentPayload.Phone,
	}
	foundFields, err := checkPatchFields("agents", "id", agentModel.ID, uniques)
	if err != nil {
		return returnInvalidResponse(http.StatusUnprocessableEntity, validate, "validation error : "+foundFields)
	}

	if len(agentPayload.Email) > 0 {
		agentModel.Email = agentPayload.Email
	}

	if len(agentPayload.Phone) > 0 {
		agentModel.Phone = agentPayload.Phone
	}

	//if payload not 0 and category must "agent" not "account_executive"
	if len(agentPayload.Banks) > 0 && agentModel.Category != "account_executive" {

		type Result struct {
			Counter int
		}
		var counter Result
		var i int

		//generate values emtity
		values := ""
		for _, val := range agentPayload.Banks {
			//auto convert to int
			if i != 0 {
				values += fmt.Sprintf(", (%d)", val)
			} else {
				values += fmt.Sprintf("(%d)", val)
			}
			i++
		}

		//query for checking not exist bank id (not valid bank id)
		db := asira.App.DB.Raw(
			fmt.Sprintf(`
			SELECT COUNT(t.id) AS counter
			FROM (
			VALUES %s 
			) AS t(id)
			LEFT JOIN banks b on b.id = t.id
			where b.id is null
			`, values)).Scan(&counter)
		err = db.Error
		fmt.Println("counter : ", counter.Counter)
		if counter.Counter != 0 {
			return returnInvalidResponse(http.StatusUnprocessableEntity, validate, "validation error : invalid banks id")
		}
		agentModel.Banks = pq.Int64Array(agentPayload.Banks)
	}

	if len(agentPayload.Image) > 0 {

		//upload image id card
		url, err := uploadImageS3Formatted("agt", agentPayload.Image)
		if err != nil {
			return returnInvalidResponse(http.StatusInternalServerError, err, "Gagal upload foto agent")
		}

		//DONE: delete old image
		if len(agentModel.Image) > 0 {
			err = deleteImageS3(agentModel.Image)
			if err != nil {
				return returnInvalidResponse(http.StatusInternalServerError, err, "Gagal menghapus foto lama agent")
			}
		}

		agentModel.Image = url
	}
	//restoring old password and update data
	agentModel.Password = password
	err = agentModel.Save()
	if err != nil {
		return returnInvalidResponse(http.StatusUnprocessableEntity, err, "Gagal mengubah data akun agen")
	}

	//Refetching after update
	var response AgentResponse
	db := asira.App.DB.Table("agents ag").
		Select("ag.*, (SELECT ARRAY_AGG(name) FROM banks WHERE id IN (SELECT UNNEST(ag.banks))) as bank_names").
		Where("ag.id = ?", agentID)
	err = db.Find(&response).Error
	if err != nil {
		return returnInvalidResponse(http.StatusForbidden, err, "Akun tidak ditemukan")
	}

	return c.JSON(http.StatusOK, response)
}
