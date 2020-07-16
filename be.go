package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	tinycrypt "github.com/uclaacm/teach-la-go-backend-tinycrypt"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	aliasPath  = "alias"  // alias collection name
	cipherPath = "cipher" // cipher collection name
	aliasID    = "ID"     // name of field to store aliased ID in doc
	cfgVar     = "CREDS"  // environment variable holding firebase credentials
)

// Cipher from the frontend
type Cipher struct {
	ShAmt     int64  `json:"shamt" firestore:"shamt"`
	Plaintext string `json:"plaintext" firestore:"plaintext"`
}

func main() {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// set up the app through which our client will be
	// acquired.
	opt := option.WithCredentialsJSON([]byte(os.Getenv(cfgVar)))
	app, appErr := firebase.NewApp(context.Background(), nil, opt)
	d, cliErr := app.Firestore(context.Background())
	if appErr != nil || cliErr != nil {
		e.Logger.Fatal("failed to create firebase connection")
	}

	// return the cipher with provided wordHash.
	e.GET("/cipher/:wordHash", func(c echo.Context) error {
		cipher := Cipher{}

		err := d.RunTransaction(c.Request().Context(), func(ctx context.Context, tx *firestore.Transaction) error {
			aref := d.Collection(aliasPath).Doc(c.Param("wordHash"))
			asnap, err := aref.Get(ctx)
			if err != nil {
				return err
			}
			id, err := asnap.DataAt(aliasID)
			if err != nil {
				return err
			}
			csnap, err := d.Collection(cipherPath).Doc(id.(string)).Get(ctx)
			if err != nil {
				return err
			}
			return csnap.DataTo(&cipher)
		})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return c.String(http.StatusNotFound, "could not find the cipher doc or alias")
			}
			return c.String(http.StatusInternalServerError, errors.Wrap(err, "could not get the cipher").Error())
		}
		return c.JSON(http.StatusOK, &cipher)
	})

	// create a new cipher from provided body.
	e.POST("/cipher", func(c echo.Context) error {
		cipher := Cipher{}
		if c.Bind(&cipher) != nil {
			return c.String(http.StatusBadRequest, "request body is ill-formatted!")
		}

		wordHash := ""

		err := d.RunTransaction(c.Request().Context(), func(ctx context.Context, tx *firestore.Transaction) error {
			// cipher doc
			cref := d.Collection(cipherPath).NewDoc()
			if err := tx.Create(cref, &cipher); err != nil {
				return err
			}

			// create our alias
			wordHash = strings.Join(tinycrypt.GenerateWord36(tinycrypt.MakeHash(cref.ID)), "-")
			aref := d.Collection(aliasPath).Doc(wordHash)
			aliasContent := make(map[string]string)
			aliasContent[aliasID] = cref.ID
			return tx.Create(aref, &aliasContent)
		})
		if err != nil {
			return c.String(http.StatusInternalServerError, errors.Wrap(err, "failed to create cipher").Error())
		}
		return c.String(http.StatusCreated, wordHash)
	})

	e.Logger.Fatal(e.Start(":8081"))
}
