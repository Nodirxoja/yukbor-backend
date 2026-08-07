#!/usr/bin/env bash
# Narrated walkthrough of the whole system (plan §9): living documentation and
# a pre-demo sanity check in one. Every call goes through the gateway exactly
# as the iOS app would.
#
#   ./scripts/demo.sh              step through, pausing between sections
#   ./scripts/demo.sh -y           run straight through (CI / quick check)
#   ./scripts/demo.sh -y http://host:8080
set -uo pipefail

AUTO=0
[ "${1:-}" = "-y" ] && { AUTO=1; shift; }
BASE="${1:-http://localhost:8080}"

SELFIE="$(mktemp -t yukbor-selfie).jpg"
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xd9' > "$SELFIE"
trap 'rm -f "$SELFIE"' EXIT

RUN=$RANDOM
PASS=0; FAIL=0

bold()  { printf '\n\033[1m%s\033[0m\n' "$1"; }
note()  { printf '  \033[2m%s\033[0m\n' "$1"; }
pause() { [ $AUTO -eq 1 ] || { printf '\n  \033[2m[enter]\033[0m'; read -r _; }; }

j() { python3 -c "import sys,json
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for k in '$1'.split('.'):
    d = d[int(k)] if k.isdigit() else d.get(k,'')
    if d=='': break
print(d if d is not None else '')" 2>/dev/null; }

# expect <label> <expected> <actual>
expect() {
  if [ "$2" = "$3" ]; then
    printf '  \033[32mok\033[0m    %-46s %s\n' "$1" "$3"; PASS=$((PASS+1))
  else
    printf '  \033[31mFAIL\033[0m  %-46s got %-22s want %s\n' "$1" "${3:-<empty>}" "$2"; FAIL=$((FAIL+1))
  fi
}
code() { printf '%s' "$1" | j error.code; }

register() { # phone pinfl passport role name -> id<TAB>token
  local vid c t out
  out=$(curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"$1\"}")
  vid=$(printf '%s' "$out" | j verificationId); c=$(printf '%s' "$out" | j devCode)
  curl -sS -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' -d "{\"verificationId\":\"$vid\",\"code\":\"$c\"}" >/dev/null
  t=$(curl -sS -X POST "$BASE/auth/myid/verify" -F "verificationId=$vid" -F passportSeries=AB \
      -F "passportNumber=$3" -F "pinfl=$2" -F birthDate=1990-05-14 -F "selfie=@$SELFIE" | j myIdVerificationToken)
  out=$(curl -sS -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
        -d "{\"fullName\":\"$5\",\"phoneNumber\":\"$1\",\"role\":\"$4\",\"myIdVerificationToken\":\"$t\"}")
  printf '%s\t%s\t%s\n' "$(printf '%s' "$out" | j user.id)" "$(printf '%s' "$out" | j accessToken)" "$(printf '%s' "$out" | j user.fullName)"
}

curl -fsS --max-time 5 "$BASE/health" >/dev/null || { echo "gateway not reachable at $BASE — run 'make up'"; exit 1; }

# =======================================================================
bold "1 · Registration: phone OTP, then mandatory MyID KYC"
note "Nothing is stubbed: the OTP is hashed and rate-limited, the KYC token"
note "has a real TTL and is consumed exactly once, the licence is issued by"
note "the registry. Only the upstream is simulated."

IFS=$'\t' read -r CLIENT_ID CLIENT_T CLIENT_NAME <<< "$(register "+9989${RUN:0:4}0101" '46701987654322' '9100001' 'client' 'Введённое имя')"
expect "client registered" "$CLIENT_ID" "$CLIENT_ID"
note "MyID overrode the typed name with the official one: $CLIENT_NAME"

IFS=$'\t' read -r DRV_ID DRV_T DRV_NAME <<< "$(register "+9989${RUN:0:4}0102" '35503987654328' '9100002' 'driver' 'Водитель')"
note "driver: $DRV_NAME — licence B/C/CE"
IFS=$'\t' read -r DRV2_ID DRV2_T DRV2_NAME <<< "$(register "+9989${RUN:0:4}0103" '33409876543214' '9100003' 'driver' 'Водитель 2')"
note "driver: $DRV2_NAME — licence B/C only"
IFS=$'\t' read -r EQ_ID EQ_T EQ_NAME <<< "$(register "+9989${RUN:0:4}0104" '36607654321096' '9100004' 'equipmentProvider' 'Техника')"
IFS=$'\t' read -r ADM_ID ADM_T ADM_NAME <<< "$(register "+9989${RUN:0:4}0109" '38801234567896' '9100009' 'admin' 'Админ')"
pause

# =======================================================================
bold "2 · The rejection paths — every one of them runs for real"
V=$(curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"+9989${RUN:0:4}0201\"}")
VID=$(printf '%s' "$V" | j verificationId)
curl -sS -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' -d "{\"verificationId\":\"$VID\",\"code\":\"$(printf '%s' "$V" | j devCode)\"}" >/dev/null

OUT=$(curl -sS -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=0000000 -F pinfl=32109876543218 -F birthDate=1990-01-01 -F "selfie=@$SELFIE")
expect "passport 0000000 in the registry" PASSPORT_NOT_FOUND "$(code "$OUT")"

OUT=$(curl -sS -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=1234567 -F pinfl=99109876543218 -F birthDate=1990-01-01 -F "selfie=@$SELFIE")
expect "selfie does not match the document" FACE_MISMATCH "$(code "$OUT")"

OUT=$(curl -sS -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
      -d "{\"fullName\":\"X\",\"phoneNumber\":\"+9989${RUN:0:4}0201\",\"role\":\"client\",\"myIdVerificationToken\":\"myid_tok_expired\"}")
expect "registering with a bogus KYC token" MYID_TOKEN_EXPIRED_OR_INVALID "$(code "$OUT")"

OUT=$(register "+9989${RUN:0:4}0202" '37701234567891' '9100011' 'driver' 'Без категории' 2>/dev/null; \
      curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d '{"phoneNumber":"+998000"}')
expect "malformed phone number" VALIDATION_ERROR "$(code "$OUT")"

# The licence rejection, run cleanly so the code is visible.
V=$(curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' -d "{\"phoneNumber\":\"+9989${RUN:0:4}0203\"}")
VID=$(printf '%s' "$V" | j verificationId)
curl -sS -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' -d "{\"verificationId\":\"$VID\",\"code\":\"$(printf '%s' "$V" | j devCode)\"}" >/dev/null
TOK=$(curl -sS -X POST "$BASE/auth/myid/verify" -F "verificationId=$VID" -F passportSeries=AB \
      -F passportNumber=9100012 -F pinfl=37701234567893 -F birthDate=1990-01-01 -F "selfie=@$SELFIE" | j myIdVerificationToken)
OUT=$(curl -sS -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
      -d "{\"fullName\":\"Без категории\",\"phoneNumber\":\"+9989${RUN:0:4}0203\",\"role\":\"driver\",\"myIdVerificationToken\":\"$TOK\"}")
expect "driver whose licence has no category C" LICENSE_CATEGORY_MISMATCH "$(code "$OUT")"
note "That applicant is stored as 'rejected' with a reason — visible in the dashboard."
pause

# =======================================================================
bold "3 · Pricing happens on the server, and it splits per leg"
BODY="{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportWithOptions\",
 \"cargo\":{\"cargoType\":\"Стройматериалы\",\"weightTons\":12.5,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"flatbed\"},
 \"equipmentRequest\":{\"equipmentType\":\"crane\",\"durationHours\":4},
 \"laborRequest\":{\"workersCount\":3,\"durationHours\":2},
 \"pickupAddress\":\"Ташкент, Амира Темура 15\",\"pickupLocation\":{\"latitude\":41.311081,\"longitude\":69.240562},
 \"dropoffAddress\":\"Самарканд, Регистан 5\",\"dropoffLocation\":{\"latitude\":39.654896,\"longitude\":66.959843},
 \"scheduledDate\":\"2026-08-10T09:00:00Z\",\"currency\":\"UZS\"}"
curl -sS -X POST "$BASE/orders/estimate" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' \
  -d "$BODY" | python3 -m json.tool | sed 's/^/  /'
note "Tariffs live in orders.tariffs — they can be changed live, no rebuild."
pause

# =======================================================================
bold "4 · One order, three legs, three independent executors"
ORDER=$(curl -sS -X POST "$BASE/orders" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' -d "$BODY" | j id)
expect "combo order created" "$ORDER" "$ORDER"

OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/accept" -H "Authorization: Bearer $DRV_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"executorId\":\"$DRV_ID\",\"executorName\":\"$DRV_NAME\"}")
expect "driver claims the transport leg" accepted "$(printf '%s' "$OUT" | j status)"

OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/accept" -H "Authorization: Bearer $DRV2_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"executorId\":\"$DRV2_ID\",\"executorName\":\"$DRV2_NAME\"}")
expect "a second driver races for the same leg" LEG_ALREADY_TAKEN "$(code "$OUT")"
note "One conditional UPDATE decides it — the loser cannot also win."

OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/accept" -H "Authorization: Bearer $EQ_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"equipment\",\"executorId\":\"$EQ_ID\",\"executorName\":\"$EQ_NAME\"}")
expect "equipment provider claims their own leg" accepted "$(printf '%s' "$OUT" | j equipmentStatus)"

OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/accept" -H "Authorization: Bearer $DRV2_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"labor\",\"executorId\":\"$DRV2_ID\",\"executorName\":\"Бригада\"}")
expect "labor leg claimed" accepted "$(printf '%s' "$OUT" | j laborStatus)"
pause

# =======================================================================
bold "5 · Escrow opened separately for each executor"
for pair in "transport:$DRV_ID" "equipment:$EQ_ID" "labor:$DRV2_ID"; do
  curl -sS "$BASE/wallet/transactions?payeeId=${pair#*:}" -H "Authorization: Bearer $ADM_T" \
    | python3 -c "import sys,json
rows=[t for t in json.load(sys.stdin) if t['orderId']=='$ORDER']
for t in rows:
    print(f\"  {'${pair%%:*}':<11}{int(t['amount']):>10,} UZS   fee {int(t['platformCommission']):>8,}   {t['status']:<7} {t['providerRef']}\")" 2>/dev/null
done
curl -sS "$BASE/wallet/transactions?payeeId=$DRV_ID" -H "Authorization: Bearer $ADM_T" >/dev/null
note "Three rows for ONE order — the escrow key is (orderId, payeeId)."
note "The order total is 3 070 000; the legs add up to exactly that."
pause

# =======================================================================
bold "6 · The status walk, enforced forward-only"
for s in loadingInProgress inTransit delivered; do
  R=$(curl -sS -X PATCH "$BASE/orders/$ORDER/status" -H "Authorization: Bearer $DRV_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"status\":\"$s\"}" | j status)
  expect "transport -> $s" "$s" "$R"
done
OUT=$(curl -sS -X PATCH "$BASE/orders/$ORDER/status" -H "Authorization: Bearer $DRV_T" -H 'Content-Type: application/json' \
      -d '{"leg":"transport","status":"inTransit"}')
expect "driver tries to go BACKWARDS" INVALID_STATUS_TRANSITION "$(code "$OUT")"
OUT=$(curl -sS -X PATCH "$BASE/orders/$ORDER/status" -H "Authorization: Bearer $EQ_T" -H 'Content-Type: application/json' \
      -d '{"leg":"transport","status":"delivered"}')
expect "someone else's leg" FORBIDDEN "$(code "$OUT")"

OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/confirm-completion" -H "Authorization: Bearer $CLIENT_T")
expect "client confirms while 2 legs are unfinished" ORDER_NOT_READY "$(code "$OUT")"

curl -sS -X PATCH "$BASE/orders/$ORDER/status" -H "Authorization: Bearer $EQ_T" -H 'Content-Type: application/json' -d '{"leg":"equipment","status":"delivered"}' >/dev/null
curl -sS -X PATCH "$BASE/orders/$ORDER/status" -H "Authorization: Bearer $DRV2_T" -H 'Content-Type: application/json' -d '{"leg":"labor","status":"delivered"}' >/dev/null
pause

# =======================================================================
bold "7 · Client confirms — every executor is paid independently"
OUT=$(curl -sS -X POST "$BASE/orders/$ORDER/confirm-completion" -H "Authorization: Bearer $CLIENT_T")
expect "all legs completed" completed "$(printf '%s' "$OUT" | j status)"
curl -sS "$BASE/wallet/transactions?payeeId=$DRV_ID" -H "Authorization: Bearer $ADM_T" \
  | python3 -c "import sys,json
for t in json.load(sys.stdin):
    print(f\"  driver paid {int(t['amount'])-int(t['platformCommission']):>10,} UZS   platform kept {int(t['platformCommission']):>9,}   {t['status']}\")" 2>/dev/null
curl -sS -X POST "$BASE/orders/$ORDER/confirm-completion" -H "Authorization: Bearer $CLIENT_T" >/dev/null
note "Confirming twice changes nothing — release is idempotent."
pause

# =======================================================================
bold "8 · Reviews feed the rating"
OUT=$(curl -sS -X POST "$BASE/reviews" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' \
      -d "{\"orderId\":\"$ORDER\",\"revieweeId\":\"$DRV_ID\",\"rating\":5,\"comment\":\"Всё отлично\"}")
expect "review accepted after completion" 5 "$(printf '%s' "$OUT" | j rating)"
OUT=$(curl -sS -X POST "$BASE/reviews" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' \
      -d "{\"orderId\":\"$ORDER\",\"revieweeId\":\"$DRV_ID\",\"rating\":1}")
expect "reviewing the same order twice" REVIEW_ALREADY_EXISTS "$(code "$OUT")"
expect "rating landed on the driver's profile" 5 "$(curl -sS "$BASE/users/me" -H "Authorization: Bearer $DRV_T" | j rating)"
pause

# =======================================================================
bold "9 · The licence is checked AGAIN when the load demands more"
TT=$(curl -sS -X POST "$BASE/orders" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' -d "{
 \"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Металлопрокат\",\"weightTons\":22,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"tractorTrailer\"},
 \"pickupAddress\":\"Ташкент\",\"pickupLocation\":{\"latitude\":41.31,\"longitude\":69.24},
 \"dropoffAddress\":\"Андижан\",\"dropoffLocation\":{\"latitude\":40.78,\"longitude\":72.34},
 \"scheduledDate\":\"2026-08-12T06:00:00Z\",\"currency\":\"UZS\"}" | j id)
OUT=$(curl -sS -X POST "$BASE/orders/$TT/accept" -H "Authorization: Bearer $DRV2_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"executorId\":\"$DRV2_ID\",\"executorName\":\"$DRV2_NAME\"}")
expect "driver with C but not CE takes a trailer load" LICENSE_CATEGORY_MISMATCH "$(code "$OUT")"
OUT=$(curl -sS -X POST "$BASE/orders/$TT/accept" -H "Authorization: Bearer $DRV_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"executorId\":\"$DRV_ID\",\"executorName\":\"$DRV_NAME\"}")
expect "driver with CE takes the same load" accepted "$(printf '%s' "$OUT" | j status)"
pause

# =======================================================================
bold "10 · Payment declined — and the leg is given back"
D=$(curl -sS -X POST "$BASE/orders" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' -d "{
 \"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Тест\",\"weightTons\":1,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"flatbed\"},
 \"pickupAddress\":\"Ташкент\",\"pickupLocation\":{\"latitude\":41.3,\"longitude\":69.2},
 \"dropoffAddress\":\"Ташкент\",\"dropoffLocation\":{\"latitude\":41.4,\"longitude\":69.3},
 \"scheduledDate\":\"2026-08-12T09:00:00Z\",\"priceEstimate\":\"999999999\",\"currency\":\"UZS\"}" | j id)
OUT=$(curl -sS -X POST "$BASE/orders/$D/accept" -H "Authorization: Bearer $DRV_T" -H 'Content-Type: application/json' \
      -d "{\"leg\":\"transport\",\"executorId\":\"$DRV_ID\",\"executorName\":\"$DRV_NAME\"}")
expect "provider declines the charge" PAYMENT_DECLINED "$(code "$OUT")"
expect "the leg is published again, not stuck" published \
  "$(curl -sS "$BASE/orders/$D" -H "Authorization: Bearer $CLIENT_T" | j status)"
# Tidy up: leaving an unpayable order in the feed would read as a bug.
curl -sS -X POST "$BASE/orders/$D/cancel" -H "Authorization: Bearer $CLIENT_T" >/dev/null
pause

# =======================================================================
bold "11 · Backhaul — the driver is in Samarkand and does not go home empty"
N=$(curl -sS "$BASE/orders/backhaul?dropoffLat=39.654896&dropoffLng=66.959843&excludeOrderId=$ORDER" \
   -H "Authorization: Bearer $DRV_T" | python3 -c "import sys,json;print(len(json.load(sys.stdin)))" 2>/dev/null)
note "$N open loads within 15 km of the drop-off point, nearest first"
curl -sS "$BASE/orders/backhaul?dropoffLat=39.654896&dropoffLng=66.959843" -H "Authorization: Bearer $DRV_T" \
  | python3 -c "import sys,json
for o in json.load(sys.stdin)[:5]:
    print(f\"  {o['pickupAddress'][:44]:<46} {int(o['priceEstimate']):>10,} UZS\")" 2>/dev/null
pause

# =======================================================================
bold "12 · Mission control"
curl -sS "$BASE/admin/stats" -H "Authorization: Bearer $ADM_T" | python3 -m json.tool | sed 's/^/  /'

printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
