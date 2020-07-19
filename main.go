package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"
)

//Ackdata for ack
type Ackdata struct {
	AckID string `json:"ack"`
}
type ackresponse struct {
	Status string  `json:"status" default:"SUCCESS"`
	Data   Ackdata `json:"data"`
}
type acknowledgment struct {
	AckID   string `json:"ackID"`
	TranxID string `json:"tranxID"`
	TransID string `json:"tID"`
}

const (
	auther    = "auth-error"
	apierr    = "invalid-api-parameters"
	referr    = "invalid-ref-id"
	amterr    = "amount-mismatch"
	patherr   = "path-not-found"
	unhanderr = "unhandled-error"
	erromsg   = "ERROR"
)

type user struct {
	Name         string `json:"name" form:"name" query:"name"`
	MobileNumber string `json:"mobileNumber" form:"mobileNumber" query:"mobileNumber"`
}

type bill struct {
	CustomerName string `json:"customerName,omitempty" form:"customerName" query:"customerName"`
	DueAmount    string `json:"dueAmount,omitempty" form:"dueAmount" query:"dueAmount"`
	DueDate      string `json:"dueDate,omitempty" form:"dueDate" query:"dueDate"`
	RefID        string `json:"refID,omitempty" form:"refID" query:"refID"`
}

type msg struct {
	Status    string `json:"status" form:"status" query:"status"`
	ErrorCode string `json:"errorCode,omitempty" form:"errorCode" query:"errorCode"`
	Data      bill   `json:"data,omitempty" form:"data" query:"data,omitempty"`
}
type emsg struct {
	Status    string `json:"status" form:"status" query:"status"`
	ErrorCode string `json:"errorCode,omitempty" form:"errorCode" query:"errorCode"`
}

type transaction struct {
	AmountPaid string `json:"amountPaid" form:"amountPaid" query:"amountPaid"`
	Date       string `json:"date,omitempty" form:"date" query:"date"`
	ID         string `json:"id,omitempty" form:"id" query:"id"`
}
type payment struct {
	RefID       string      `json:"refID" form:"refID" query:"refID"`
	Transaction transaction `json:"transaction,omitempty" form:"transaction" query:"transaction"`
}

var client *mongo.Client
var err error
var ctx context.Context
var db *mongo.Database

// FetchBill will add a new Donor
func FetchBill(c echo.Context) (err error) {
	u := new(user)
	finalresponse := new(msg)
	if err = c.Bind(u); err != nil {
		log.Println(err)
		return
	}
	mn := u.MobileNumber

	if u.MobileNumber == "" {
		return c.JSON(http.StatusBadRequest, emsg{
			erromsg,
			apierr,
		})
	}
	filter := bson.M{
		"mobileNumber": bson.M{
			"$eq": mn,
		},
	}
	coll := db.Collection("users")
	// var res []bson.M
	var response msg
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		// log.Println(err)
		response.Status = "ERROR"
		response.ErrorCode = "unhandled-error"
		return c.JSON(http.StatusInternalServerError, response)
	}
	if cursor.Next(ctx) == false {
		response.Status = "ERROR"
		response.ErrorCode = "customer-not-found"
		return c.JSON(http.StatusNotFound, response)
	}
	cursor, err = coll.Find(context.TODO(), filter)
	for cursor.Next(ctx) {

		var result user
		err := cursor.Decode(&result)
		if err != nil {
			fmt.Println("cursor.Next() error:", err)

		} else {
			// res = append(res, result)
			col := db.Collection("bills")
			filter := bson.M{
				"customerName": bson.M{
					"$eq": &result.Name,
				},
			}
			cur, e := col.Find(context.TODO(), filter)
			if e != nil {
				fmt.Println("Error")
				response.Status = "ERROR"
				response.ErrorCode = "unhandled-error"
				return c.JSON(http.StatusInternalServerError, response)
			}
			for cur.Next(ctx) {
				var billres bill
				err := cur.Decode(&billres)
				if err != nil {
					fmt.Println("cursor.Next() error:", err)
				} else {
					finalresponse.Data = billres
				}
			}
		}
		finalresponse.Status = "SUCCESS"
		fmt.Println(finalresponse.Data)
	}
	return c.JSON(http.StatusOK, finalresponse)
}

// PaymentUpdate will add a new Donor
func PaymentUpdate(c echo.Context) (err error) {
	u := new(payment)
	res1 := new(ackresponse)
	var trans acknowledgment
	// finalresponse := new(msg)
	if err = c.Bind(u); err != nil {
		log.Println(err)
		return
	}
	if (*u == payment{}) {
		return c.JSON(http.StatusBadRequest, emsg{
			erromsg,
			apierr,
		})
	}
	mn := u.RefID
	tid := u.Transaction.ID
	filter := bson.M{
		"refID": bson.M{
			"$eq": mn,
		},
	}
	filter1 := bson.M{
		"tranxid": bson.M{
			"$eq": mn,
		},
	}
	col := db.Collection("transaction")
	cur, er := col.Find(context.TODO(), filter1)
	if er != nil {
		return c.JSON(http.StatusInternalServerError, emsg{
			erromsg,
			unhanderr,
		})
	}
	coll := db.Collection("bills")
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil || (cursor.Next(ctx) == false && cur.Next(ctx) == false) {
		// log.Println(err)
		return c.JSON(http.StatusNotFound, emsg{
			erromsg,
			referr,
		})
	}
	cur, er = col.Find(context.TODO(), filter1)
	cursor, err = coll.Find(context.TODO(), filter)
	for cursor.Next(ctx) {
		var result bill
		err := cursor.Decode(&result)
		if err != nil {
			fmt.Println("cursor.Next() error:", err)

		} else {
			if result.DueAmount != u.Transaction.AmountPaid && result.DueAmount != "0" {

				return c.JSON(http.StatusBadRequest, emsg{
					erromsg,
					amterr,
				})
			}
			if result.DueAmount == "0" {
				break
			}
			update := bson.M{"$set": bson.M{"dueAmount": "0"}}
			coll.UpdateOne(context.Background(), filter, update)
			// coll.DeleteOne(context.TODO(), filter)
			trans.TranxID = u.RefID
			rand.Seed(time.Now().UnixNano())
			chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZÅÄÖ" +
				"0123456789")
			length := 10
			var b strings.Builder
			for i := 0; i < length; i++ {
				b.WriteRune(chars[rand.Intn(len(chars))])
			}
			str := b.String() // E.g. "ExcbsVQs"
			trans.AckID = str
			res1.Data.AckID = trans.AckID
			trans.TransID = tid
			log.Println(trans)
			res1.Status = "SUCCESS"
			col.InsertOne(context.TODO(), &trans)
			return c.JSON(http.StatusOK, *res1)
		}
	}
	for cur.Next(ctx) {
		var result acknowledgment
		err := cur.Decode(&result)
		if err != nil {
			fmt.Println("cursor.Next() error:", err)
		} else {
			cur.Decode(&trans)
			if trans.TransID != u.Transaction.ID || trans.TranxID != u.RefID {
				return c.JSON(http.StatusNotFound, emsg{
					erromsg,
					referr,
				})
			}
			res1.Data.AckID = result.AckID
			res1.Status = "SUCCESS"
			return c.JSON(http.StatusOK, *res1)
		}
	}
	return c.JSON(http.StatusOK, *res1)
}
func nopage(c echo.Context) (err error) {
	return c.JSON(http.StatusNotFound, emsg{
		erromsg,
		patherr,
	})
}
func main() {
	client, err = mongo.NewClient(options.Client().ApplyURI("mongodb+srv://admin:admin@testcluster-2sjwn.mongodb.net/setu?retryWrites=true&w=majority"))
	if err != nil {
		panic(err)
	}
	ctx, er := context.WithTimeout(context.Background(), 10*time.Second)
	_ = er
	err = client.Connect(ctx)
	if err != nil {
		panic(err)
	}
	db = client.Database("setu")
	e := echo.New()
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		KeyLookup: "header:x-api-key",
		Validator: func(key string, c echo.Context) (bool, error) {
			if key != "valid-key" {
				return key == "valid-key", c.JSON(http.StatusForbidden, emsg{
					erromsg,
					auther,
				})
			}
			return key == "valid-key", nil
		}}), middleware.CORSWithConfig(middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))
	e.POST("/api/v1/fetch-bill", FetchBill)
	e.POST("/api/v1/payment-update", PaymentUpdate)
	e.POST("/*", nopage)
	e.Logger.Fatal(e.Start(":" + os.Getenv("PORT")))
	log.Println("Connected to MongDb")
}
