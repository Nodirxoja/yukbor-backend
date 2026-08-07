#!/usr/bin/env bash
# Exercises every endpoint the iOS app uses, against a running deployment.
#
# This is the contract check: it walks the app's real journeys — register,
# browse, accept, drive, get paid, review — and asserts both the happy path and
# the refusals. It creates throwaway users on a +99893xxxxxxx prefix; that is
# unavoidable when testing registration for real, and is the point.
#
#   ./scripts/test-mobile-api.sh                      against production
#   ./scripts/test-mobile-api.sh http://localhost:8080
set -uo pipefail

BASE="${1:-https://yukbor.duckdns.org}"
# A valid Uzbek number is exactly +998 plus nine digits. "+99893" already
# spends two of them, so the run id must be 5 digits and the suffix 2.
RUN=$(date +%s | tail -c 6 | tr -d "\n")
PASS=0
FAIL=0
FAILED_NAMES=()

SELFIE="$(mktemp -t yukbor-selfie).jpg"
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xd9' > "$SELFIE"
trap 'rm -f "$SELFIE"' EXIT

bold() { printf '\n\033[1m%s\033[0m\n' "$1"; }
note() { printf '  \033[2m%s\033[0m\n' "$1"; }

# ok <label> <expected> <actual>
ok() {
  if [ "$2" = "$3" ]; then
    printf '  \033[32m✓\033[0m %-52s %s\n' "$1" "$3"
    PASS=$((PASS + 1))
  else
    printf '  \033[31m✗\033[0m %-52s got %-24s want %s\n' "$1" "${3:-<empty>}" "$2"
    FAIL=$((FAIL + 1))
    FAILED_NAMES+=("$1")
  fi
}

# okc <label> <expected-http> <method> <path> [curl args...]
okc() {
  local label=$1 want=$2 method=$3 path=$4
  shift 4
  local code
  code=$(curl -s -o /tmp/api_body.json -w '%{http_code}' -X "$method" "$BASE$path" "$@")
  ok "$label" "$want" "$code"
}

j() {
  python3 -c "import sys,json
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for k in '$1'.split('.'):
    d = d[int(k)] if k.isdigit() else d.get(k, '')
    if d == '': break
print(d if d is not None else '')" 2>/dev/null
}
code_of() { printf '%s' "$1" | j error.code; }
count_of() { python3 -c "import sys,json;print(len(json.load(sys.stdin)))" 2>/dev/null; }

AUTH() { printf 'Authorization: Bearer %s' "$1"; }

# register <phone> <pinfl> <passport> <role> -> "id<TAB>access<TAB>refresh<TAB>name"
register() {
  local phone=$1 pinfl=$2 passport=$3 role=$4 vid code tok out
  out=$(curl -s -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$phone\"}")
  vid=$(printf '%s' "$out" | j verificationId)
  code=$(printf '%s' "$out" | j devCode); [ -z "$code" ] && code=0000
  curl -s -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' \
       -d "{\"verificationId\":\"$vid\",\"code\":\"$code\"}" >/dev/null
  tok=$(curl -s -X POST "$BASE/auth/myid/verify" -F "verificationId=$vid" -F passportSeries=AB \
        -F "passportNumber=$passport" -F "pinfl=$pinfl" -F birthDate=1990-05-14 -F "selfie=@$SELFIE" | j myIdVerificationToken)
  out=$(curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
        -d "{\"fullName\":\"Test User\",\"phoneNumber\":\"$phone\",\"role\":\"$role\",\"myIdVerificationToken\":\"$tok\"}")
  printf '%s\t%s\t%s\t%s\n' "$(printf '%s' "$out" | j user.id)" "$(printf '%s' "$out" | j accessToken)" \
         "$(printf '%s' "$out" | j refreshToken)" "$(printf '%s' "$out" | j user.fullName)"
}

echo "mobile API check — $BASE"
curl -fsS --max-time 8 "$BASE/health" >/dev/null || { echo "gateway unreachable"; exit 1; }

# =======================================================================
bold "§1  Authentication"

P_CLIENT="+99893${RUN}01"
P_DRIVER="+99893${RUN}02"
P_EQUIP="+99893${RUN}03"

OUT=$(curl -s -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$P_CLIENT\"}")
VID=$(printf '%s' "$OUT" | j verificationId)
ok "POST /auth/otp/request returns a verificationId" "yes" "$([ -n "$VID" ] && echo yes)"
ok "  expiresInSeconds is present" "120" "$(printf '%s' "$OUT" | j expiresInSeconds)"
ok "  devCode withheld in production" "yes" "$(printf '%s' "$OUT" | j devCode | grep -q . && echo no || echo yes)"

okc "POST /auth/otp/verify accepts the code" 200 POST /auth/otp/verify \
  -H 'Content-Type: application/json' -d "{\"verificationId\":\"$VID\",\"code\":\"0000\"}"
ok "  responds {verified:true}" "True" "$(j verified < /tmp/api_body.json)"

# A FRESH challenge: re-verifying an already-confirmed id is deliberately
# idempotent and returns 200, so testing a wrong code against $VID would be
# asserting the opposite of the intended behaviour.
BADV=$(curl -s -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' \
       -d "{\"phoneNumber\":\"$P_CLIENT\"}" | j verificationId)
OUT=$(curl -s -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' \
      -d "{\"verificationId\":\"$BADV\",\"code\":\"1357\"}")
ok "  a wrong code is refused" "OTP_INVALID" "$(code_of "$OUT")"
OUT=$(curl -s -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' \
      -d "{\"verificationId\":\"$VID\",\"code\":\"1357\"}")
ok "  re-verifying a confirmed id stays 200 (idempotent)" "True" "$(printf '%s' "$OUT" | j verified)"

OUT=$(curl -s -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=1234567 -F pinfl=46701987654322 -F birthDate=1990-05-14 -F "selfie=@$SELFIE")
MYID=$(printf '%s' "$OUT" | j myIdVerificationToken)
ok "POST /auth/myid/verify issues a token" "yes" "$([ -n "$MYID" ] && echo yes)"
ok "  isMatched true" "True" "$(printf '%s' "$OUT" | j isMatched)"
ok "  returns the official name" "yes" "$(printf '%s' "$OUT" | j verifiedFullName | grep -q . && echo yes)"

OUT=$(curl -s -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=0000000 -F pinfl=46701987654322 -F birthDate=1990-05-14 -F "selfie=@$SELFIE")
ok "  unknown passport refused" "PASSPORT_NOT_FOUND" "$(code_of "$OUT")"
OUT=$(curl -s -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=1234567 -F pinfl=99701987654322 -F birthDate=1990-05-14 -F "selfie=@$SELFIE")
ok "  face mismatch refused" "FACE_MISMATCH" "$(code_of "$OUT")"

OUT=$(curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
      -d "{\"fullName\":\"Client\",\"phoneNumber\":\"$P_CLIENT\",\"role\":\"client\",\"myIdVerificationToken\":\"$MYID\"}")
CLIENT_ID=$(printf '%s' "$OUT" | j user.id)
CLIENT_T=$(printf '%s' "$OUT" | j accessToken)
CLIENT_R=$(printf '%s' "$OUT" | j refreshToken)
ok "POST /auth/register creates the user" "yes" "$([ -n "$CLIENT_ID" ] && echo yes)"
ok "  verificationStatus approved" "approved" "$(printf '%s' "$OUT" | j user.verificationStatus)"
ok "  returns both tokens" "yes" "$([ -n "$CLIENT_T" ] && [ -n "$CLIENT_R" ] && echo yes)"

OUT=$(curl -s -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
      -d "{\"fullName\":\"X\",\"phoneNumber\":\"$P_CLIENT\",\"role\":\"client\",\"myIdVerificationToken\":\"$MYID\"}")
ok "  a consumed MyID token is refused" "MYID_TOKEN_EXPIRED_OR_INVALID" "$(code_of "$OUT")"

IFS=$'\t' read -r DRIVER_ID DRIVER_T DRIVER_R DRIVER_NAME <<< "$(register "$P_DRIVER" 35503987654328 2223334 driver)"
IFS=$'\t' read -r EQUIP_ID EQUIP_T EQUIP_R EQUIP_NAME <<< "$(register "$P_EQUIP" 36607654321096 3334445 equipmentProvider)"
ok "  driver registered" "yes" "$([ -n "$DRIVER_ID" ] && echo yes)"
ok "  equipment provider registered" "yes" "$([ -n "$EQUIP_ID" ] && echo yes)"

# Registering verifies the phone, so login is allowed for the next 15 minutes
# without repeating the SMS — the behaviour the app depends on when a user
# reopens it shortly after signing up.
OUT=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$P_CLIENT\"}")
ok "POST /auth/login works right after registering" "yes" "$(printf '%s' "$OUT" | j accessToken | grep -q . && echo yes)"

V2=$(curl -s -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$P_CLIENT\"}" | j verificationId)
curl -s -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' -d "{\"verificationId\":\"$V2\",\"code\":\"0000\"}" >/dev/null
OUT=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$P_CLIENT\"}")
ok "POST /auth/login after OTP returns tokens" "yes" "$(printf '%s' "$OUT" | j accessToken | grep -q . && echo yes)"
CLIENT_T=$(printf '%s' "$OUT" | j accessToken)
CLIENT_R=$(printf '%s' "$OUT" | j refreshToken)

OUT=$(curl -s -X POST "$BASE/auth/login" -H 'Content-Type: application/json' -d '{"phoneNumber":"+998999999999"}')
ok "  unknown number refused" "OTP_NOT_VERIFIED" "$(code_of "$OUT")"

OUT=$(curl -s -X POST "$BASE/auth/refresh" -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$CLIENT_R\"}")
NEW_T=$(printf '%s' "$OUT" | j accessToken)
NEW_R=$(printf '%s' "$OUT" | j refreshToken)
ok "POST /auth/refresh rotates the pair" "yes" "$([ -n "$NEW_T" ] && [ -n "$NEW_R" ] && echo yes)"

# Rotation means the token just spent must be worthless — that is the whole
# point of rotating, so it is asserted rather than assumed.
OUT=$(curl -s -X POST "$BASE/auth/refresh" -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$CLIENT_R\"}")
ok "  the spent refresh token is dead" "UNAUTHORIZED" "$(code_of "$OUT")"
CLIENT_T=$NEW_T
CLIENT_R=$NEW_R

# =======================================================================
bold "§2  Profile"

okc "GET /users/me" 200 GET /users/me -H "$(AUTH "$CLIENT_T")"
ok "  returns this user" "$CLIENT_ID" "$(j id < /tmp/api_body.json)"
ok "  rating present" "0" "$(j rating < /tmp/api_body.json)"
okc "GET /users/me without a token" 401 GET /users/me
okc "PATCH /users/me updates the profile" 200 PATCH /users/me -H "$(AUTH "$CLIENT_T")" \
  -H 'Content-Type: application/json' -d '{"email":"client@yukbor.uz"}'
ok "  email persisted" "client@yukbor.uz" "$(j email < /tmp/api_body.json)"

# =======================================================================
bold "§3  Orders"

BODY='{"clientName":"Client","type":"transportWithOptions",
 "cargo":{"cargoType":"Стройматериалы","weightTons":12.5,"requiresRefrigeration":false,"requiredVehicleType":"flatbed"},
 "equipmentRequest":{"equipmentType":"crane","durationHours":4},
 "pickupAddress":"Ташкент, Амира Темура 15","pickupLocation":{"latitude":41.311081,"longitude":69.240562},
 "dropoffAddress":"Самарканд, Регистан 5","dropoffLocation":{"latitude":39.654896,"longitude":66.959843},
 "scheduledDate":"2026-09-01T09:00:00Z","currency":"UZS"}'

okc "POST /orders/estimate" 200 POST /orders/estimate -H "$(AUTH "$CLIENT_T")" \
  -H 'Content-Type: application/json' -d "$BODY"
EST=$(j priceEstimate < /tmp/api_body.json)
ok "  returns a price" "yes" "$([ -n "$EST" ] && echo yes)"
ok "  returns a per-leg breakdown" "yes" "$(j breakdown.transport < /tmp/api_body.json | grep -q . && echo yes)"

okc "POST /orders creates it" 201 POST /orders -H "$(AUTH "$CLIENT_T")" \
  -H 'Content-Type: application/json' -d "$BODY"
ORDER=$(j id < /tmp/api_body.json)
ok "  status published" "published" "$(j status < /tmp/api_body.json)"
ok "  equipmentStatus published" "published" "$(j equipmentStatus < /tmp/api_body.json)"
ok "  laborStatus null (no labor leg)" "None" "$(python3 -c "import json;print(json.load(open('/tmp/api_body.json'))['laborStatus'])" 2>/dev/null)"

okc "GET /orders/{id}" 200 GET "/orders/$ORDER" -H "$(AUTH "$CLIENT_T")"
okc "GET /orders/{unknown}" 404 GET "/orders/00000000-0000-0000-0000-000000000000" -H "$(AUTH "$CLIENT_T")"
ok "  ORDER_NOT_FOUND" "ORDER_NOT_FOUND" "$(code_of "$(cat /tmp/api_body.json)")"

okc "GET /orders?clientId=" 200 GET "/orders?clientId=$CLIENT_ID" -H "$(AUTH "$CLIENT_T")"
ok "  the new order is listed" "yes" "$(python3 -c "
import json;d=json.load(open('/tmp/api_body.json'));print('yes' if any(o['id']=='$ORDER' for o in d) else 'no')" 2>/dev/null)"
okc "GET another client's orders is refused" 403 GET "/orders?clientId=$DRIVER_ID" -H "$(AUTH "$CLIENT_T")"

okc "GET /orders/available?leg=transport" 200 GET "/orders/available?leg=transport" -H "$(AUTH "$DRIVER_T")"
ok "  returns a list" "yes" "$([ "$(count_of < /tmp/api_body.json)" -ge 0 ] && echo yes)"
okc "GET /orders/available with a bad leg" 400 GET "/orders/available?leg=nonsense" -H "$(AUTH "$DRIVER_T")"

# =======================================================================
bold "§4  Accepting and driving"

okc "POST /orders/{id}/accept (transport)" 200 POST "/orders/$ORDER/accept" -H "$(AUTH "$DRIVER_T")" \
  -H 'Content-Type: application/json' -d "{\"leg\":\"transport\",\"executorId\":\"$DRIVER_ID\",\"executorName\":\"$DRIVER_NAME\"}"
ok "  status accepted" "accepted" "$(j status < /tmp/api_body.json)"
ok "  driver is assigned" "$DRIVER_ID" "$(j assignedDriverId < /tmp/api_body.json)"

OUT=$(curl -s -X POST "$BASE/orders/$ORDER/accept" -H "$(AUTH "$EQUIP_T")" \
      -H 'Content-Type: application/json' -d "{\"leg\":\"transport\",\"executorId\":\"$EQUIP_ID\",\"executorName\":\"$EQUIP_NAME\"}")
ok "  a second executor is refused" "LEG_ALREADY_TAKEN" "$(code_of "$OUT")"

okc "POST accept (equipment leg, independent)" 200 POST "/orders/$ORDER/accept" -H "$(AUTH "$EQUIP_T")" \
  -H 'Content-Type: application/json' -d "{\"leg\":\"equipment\",\"executorId\":\"$EQUIP_ID\",\"executorName\":\"$EQUIP_NAME\"}"
ok "  equipmentStatus accepted" "accepted" "$(j equipmentStatus < /tmp/api_body.json)"

for s in loadingInProgress inTransit delivered; do
  okc "PATCH /orders/{id}/status → $s" 200 PATCH "/orders/$ORDER/status" -H "$(AUTH "$DRIVER_T")" \
    -H 'Content-Type: application/json' -d "{\"leg\":\"transport\",\"status\":\"$s\"}"
done
OUT=$(curl -s -X PATCH "$BASE/orders/$ORDER/status" -H "$(AUTH "$DRIVER_T")" \
      -H 'Content-Type: application/json' -d '{"leg":"transport","status":"inTransit"}')
ok "  going backwards is refused" "INVALID_STATUS_TRANSITION" "$(code_of "$OUT")"
OUT=$(curl -s -X PATCH "$BASE/orders/$ORDER/status" -H "$(AUTH "$EQUIP_T")" \
      -H 'Content-Type: application/json' -d '{"leg":"transport","status":"completed"}')
ok "  another executor's leg is refused" "FORBIDDEN" "$(code_of "$OUT")"

OUT=$(curl -s -X POST "$BASE/orders/$ORDER/confirm-completion" -H "$(AUTH "$CLIENT_T")")
ok "  confirming early is refused" "ORDER_NOT_READY" "$(code_of "$OUT")"

curl -s -X PATCH "$BASE/orders/$ORDER/status" -H "$(AUTH "$EQUIP_T")" \
  -H 'Content-Type: application/json' -d '{"leg":"equipment","status":"delivered"}' >/dev/null

okc "POST /orders/{id}/confirm-completion" 200 POST "/orders/$ORDER/confirm-completion" -H "$(AUTH "$CLIENT_T")"
ok "  every leg completed" "completed" "$(j status < /tmp/api_body.json)"
ok "  equipment leg completed too" "completed" "$(j equipmentStatus < /tmp/api_body.json)"

# =======================================================================
bold "§5  Wallet"

okc "GET /wallet/transactions (own ledger)" 200 GET /wallet/transactions -H "$(AUTH "$DRIVER_T")"
N=$(count_of < /tmp/api_body.json)
ok "  the driver has a transaction" "yes" "$([ "${N:-0}" -ge 1 ] && echo yes)"
ok "  it is released" "released" "$(python3 -c "
import json;d=json.load(open('/tmp/api_body.json'));print(d[0]['status'])" 2>/dev/null)"
ok "  commission is 10%" "yes" "$(python3 -c "
import json;d=json.load(open('/tmp/api_body.json'))[0]
print('yes' if int(d['platformCommission'])*10 == int(d['amount']) else 'no: '+d['platformCommission']+' of '+d['amount'])" 2>/dev/null)"
okc "GET another executor's ledger is refused" 403 GET "/wallet/transactions?payeeId=$EQUIP_ID" -H "$(AUTH "$DRIVER_T")"

# =======================================================================
bold "§6  Notifications"

okc "GET /notifications" 200 GET /notifications -H "$(AUTH "$CLIENT_T")"
NOTIF=$(python3 -c "
import json;d=json.load(open('/tmp/api_body.json'));print(d[0]['id'] if d else '')" 2>/dev/null)
ok "  the client was notified" "yes" "$([ -n "$NOTIF" ] && echo yes)"
if [ -n "$NOTIF" ]; then
  okc "PATCH /notifications/{id}/read" 204 PATCH "/notifications/$NOTIF/read" -H "$(AUTH "$CLIENT_T")"
  okc "  another user cannot mark it" 404 PATCH "/notifications/$NOTIF/read" -H "$(AUTH "$DRIVER_T")"
fi
okc "GET another user's notifications refused" 403 GET "/notifications?userId=$DRIVER_ID" -H "$(AUTH "$CLIENT_T")"

# =======================================================================
bold "§7  Reviews"

okc "POST /reviews after completion" 201 POST /reviews -H "$(AUTH "$CLIENT_T")" \
  -H 'Content-Type: application/json' \
  -d "{\"orderId\":\"$ORDER\",\"revieweeId\":\"$DRIVER_ID\",\"rating\":5,\"comment\":\"Всё отлично\"}"
OUT=$(curl -s -X POST "$BASE/reviews" -H "$(AUTH "$CLIENT_T")" -H 'Content-Type: application/json' \
      -d "{\"orderId\":\"$ORDER\",\"revieweeId\":\"$DRIVER_ID\",\"rating\":3}")
ok "  reviewing twice is refused" "REVIEW_ALREADY_EXISTS" "$(code_of "$OUT")"
OUT=$(curl -s -X POST "$BASE/reviews" -H "$(AUTH "$CLIENT_T")" -H 'Content-Type: application/json' \
      -d "{\"orderId\":\"$ORDER\",\"revieweeId\":\"$DRIVER_ID\",\"rating\":9}")
ok "  a rating outside 1..5 is refused" "VALIDATION_ERROR" "$(code_of "$OUT")"

okc "GET /reviews/rating?userId=" 200 GET "/reviews/rating?userId=$DRIVER_ID" -H "$(AUTH "$CLIENT_T")"
ok "  aggregate is 5" "5" "$(j rating < /tmp/api_body.json)"
ok "  count is 1" "1" "$(j count < /tmp/api_body.json)"
ok "  it reached the driver's profile" "5" "$(curl -s "$BASE/users/me" -H "$(AUTH "$DRIVER_T")" | j rating)"

# =======================================================================
bold "§8  Backhaul"

okc "GET /orders/backhaul" 200 GET "/orders/backhaul?dropoffLat=39.654896&dropoffLng=66.959843&excludeOrderId=$ORDER" \
  -H "$(AUTH "$DRIVER_T")"
ok "  returns a list" "yes" "$([ "$(count_of < /tmp/api_body.json)" -ge 0 ] && echo yes)"
okc "  missing coordinates rejected" 400 GET "/orders/backhaul" -H "$(AUTH "$DRIVER_T")"

# =======================================================================
bold "§9  Cancellation"

NEW=$(curl -s -X POST "$BASE/orders" -H "$(AUTH "$CLIENT_T")" -H 'Content-Type: application/json' -d "$BODY" | j id)
okc "POST /orders/{id}/cancel (untouched order)" 200 POST "/orders/$NEW/cancel" -H "$(AUTH "$CLIENT_T")"
ok "  status cancelled" "cancelled" "$(j status < /tmp/api_body.json)"
OUT=$(curl -s -X POST "$BASE/orders/$ORDER/cancel" -H "$(AUTH "$CLIENT_T")")
ok "  a completed order cannot be cancelled" "ORDER_NOT_CANCELLABLE" "$(code_of "$OUT")"

# =======================================================================
bold "§10 Logout"

okc "POST /auth/logout" 204 POST /auth/logout -H 'Content-Type: application/json' \
  -d "{\"refreshToken\":\"$DRIVER_R\"}"
OUT=$(curl -s -X POST "$BASE/auth/refresh" -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$DRIVER_R\"}")
ok "  the refresh token is revoked" "UNAUTHORIZED" "$(code_of "$OUT")"

# =======================================================================
printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  printf '\nFailed:\n'
  printf '  - %s\n' "${FAILED_NAMES[@]}"
  exit 1
fi
