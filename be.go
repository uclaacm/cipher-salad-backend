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
	aliasID    = "ID"     // name of field where we store the aliased ID
	cfgVar     = "CREDS"  // environment variable holding firebase credentials
)

// caesarDoc: a type that contains a field for the plaintext
// and a second field for the shift amount used to encode
// said plaintext.
type caesarDoc struct {
	ShAmt     int64  `json:"shamt" firestore:"shamt"`
	Plaintext string `json:"plaintext" firestore:"plaintext"`
}

func main() {
	// create our router instance...
	e := echo.New()
	e.HideBanner = true         // ...with no banner...
	e.Use(middleware.Logger())  // ...logging middleware...
	e.Use(middleware.Recover()) // ...autorecover...
	e.Use(middleware.CORS())    // ...and CORS.

	// set up the firebase app through which our client will be
	// acquired.
	opt := option.WithCredentialsJSON([]byte(os.Getenv(cfgVar)))
	app, appErr := firebase.NewApp(context.Background(), nil, opt)
	d, cliErr := app.Firestore(context.Background())
	if appErr != nil || cliErr != nil {
		e.Logger.Fatal("failed to create firebase connection")
	}

	// returns the cipher with the provided wordHash.
	e.GET("/cipher/:wordHash", func(c echo.Context) error {
		cipher := caesarDoc{}

		err := d.RunTransaction(c.Request().Context(), func(ctx context.Context, tx *firestore.Transaction) error {
			// acquire the alias who's doc ID is 'wordHash'.
			aref := d.Collection(aliasPath).Doc(c.Param("wordHash"))
			asnap, err := aref.Get(ctx)
			if err != nil {
				// fail if the document does not exist or an unexpected error occurs.
				return err
			}

			// get the ID of the doc it points to.
			id, err := asnap.DataAt(aliasID)
			if err != nil {
				return err
			}

			// acquire the doc pointed to by the alias.
			csnap, err := d.Collection(cipherPath).Doc(id.(string)).Get(ctx)
			if err != nil {
				return err
			}

			// and unpack it into our caesarDoc instance.
			return csnap.DataTo(&cipher)
		})
		if err != nil {
			// specific error if not found
			if status.Code(err) == codes.NotFound {
				return c.String(http.StatusNotFound, "could not find the cipher doc or alias")
			}
			// otherwise, generic.
			return c.String(http.StatusInternalServerError, errors.Wrap(err, "could not get the cipher").Error())
		}

		// respond with the marshaled doc.
		return c.JSON(http.StatusOK, &cipher)
	})

	// create a new cipher from provided body.
	e.POST("/cipher", func(c echo.Context) error {
		cipher := caesarDoc{}
		// fail if the request body is not a proper caesarDoc.
		if c.Bind(&cipher) != nil {
			return c.String(http.StatusBadRequest, "request body is ill-formatted!")
		}

		wordHash := ""
		err := d.RunTransaction(c.Request().Context(), func(ctx context.Context, tx *firestore.Transaction) error {
			// create the cipher doc with body contents
			cref := d.Collection(cipherPath).NewDoc()
			if err := tx.Create(cref, &cipher); err != nil {
				return err
			}

			// create our alias with a word hash made from the random doc ID.
			wordHash = strings.Join(tinycrypt.GenerateWord36(tinycrypt.MakeHash(cref.ID)), "-")
			aref := d.Collection(aliasPath).Doc(wordHash)
			aliasContent := make(map[string]string)
			aliasContent[aliasID] = cref.ID
			return tx.Create(aref, &aliasContent)
		})
		if err != nil {
			// we can only have unexpected errors for this endpoint.
			return c.String(http.StatusInternalServerError, errors.Wrap(err, "failed to create cipher").Error())
		}

		// respond with the word hash.
		return c.String(http.StatusCreated, wordHash)
	})

	// start the server
	e.Logger.Fatal(e.Start(":8081"))
}
