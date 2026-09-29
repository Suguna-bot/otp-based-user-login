# OTP Based User Login

A small full-stack assignment implementation with three distinct layers:

- `frontend/` — React + TypeScript checkout and registration UI
- `api/` — Go HTTP API
- `db/` — PostgreSQL schema

The flow follows the assignment document: registration creates a random 6-digit numeric login code; the checkout email is validated in the browser and checked in the background; registered users see an OTP modal; the user can skip the modal; a correct code creates a login session; and checkout details are saved to PostgreSQL.

## 1. Database

Create a PostgreSQL database and run:

```bash
psql "$DATABASE_URL" -f db/schema.sql
```

## 2. API

Set the database connection string:

```bash
cd api
set DATABASE_URL=postgres://postgres:postgres@localhost:5432/otp_login?sslmode=disable
set FRONTEND_URL=http://localhost:5173
set COOKIE_SECURE=false

go mod tidy
go run .
```

On macOS/Linux use `export` instead of `set`.

The API listens on `http://localhost:8080` by default.

## 3. Frontend

```bash
cd frontend
npm install
```

Create `.env`:

```env
VITE_API_URL=http://localhost:8080
```

Then:

```bash
npm run dev
```

Open the Vite URL shown in the terminal.

## 4. Test the required flow

1. Open Register.
2. Enter email, first name and last name.
3. Register and note the 6-digit code shown in the browser alert.
4. Open Checkout.
5. Type the registered email and continue entering phone/address.
6. When the email is complete, the API recognition check runs in the background.
7. Enter the registration code in the modal, or choose Skip for now.
8. After login, the user's name appears above the checkout form.
9. Submit checkout and verify the row in `checkout_orders`.

## Public deployment

A simple free-tier layout is:

- PostgreSQL: Supabase or another managed PostgreSQL provider
- API: Render, Railway, or another Go-compatible host
- Frontend: Vercel or another static frontend host

Set the frontend's `VITE_API_URL` to the deployed API URL. Set the API's `DATABASE_URL`, `FRONTEND_URL`, and `COOKIE_SECURE=true` in production.

For cross-site production deployments, use a same-site hosting arrangement where possible. If frontend and API are on different sites, cookie policy may need to be adjusted by the deployment setup.

## Repository checklist

- [x] Registration flow
- [x] Random 6-digit numeric code
- [x] Checkout form with email, phone and shipping address
- [x] Client-side real-time email format validation
- [x] Background user recognition request
- [x] OTP modal for registered users
- [x] Skip login option
- [x] OTP verification
- [x] Logged-in user's name shown on checkout
- [x] Checkout data stored in PostgreSQL
- [x] Distinct frontend, API and database layers
- [x] SQL schema checked into repository
- [x] `prompts.md` included
- [ ] Public hosting URL and GitHub collaborator access must be completed with the team's accounts
