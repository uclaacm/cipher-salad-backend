# cipher-salad-backend

A super small backend written for [cipher-salad](https://github.com/uclaacm/cipher-salad). This project makes use of another Go project developed in-house: [teach-la-go-backend-tinycrypt](https://github.com/uclaacm/teach-la-go-backend-tinycrypt/), by Tomoki of the Teach LA dev team and ACM Design. Endpoint documentation below:

## `GET /cipher/:wordHash`

### Example Request

`GET /cipher/cipher-salad-backend`

### Example Response

Status Code: 200 (OK)

```json
{
    "shamt": 12,
    "plaintext": "this is what we will encode"
}
```

All failed or invalid requests will be met with a non-200 response and an error message.

## `POST /cipher`

### Example Request

`POST /cipher`

```json
{
    "shamt": 1,
    "plaintext": "This is the content that was shifted by one."
}
```

### Example Response

Status Code: 201 (Created)

```
three-word-hash
```

This three word hash can then be used in a call to [`GET /cipher/three-word-hash`](#get-cipherwordhash).

All failed or invalid requests will be met with a non-201, non-200 response and an error message.

## Implementation Details

Aliases are actually just `map[string]string`s that hold a single value, whose field is specified through the `aliasID` constant in `be.go`.

All firebase operations are carried out in transactions.