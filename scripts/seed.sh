#!/usr/bin/env bash
# Seed believable demo data (plan §10): every screen of the iOS app and every
# panel of the dashboard shows real-looking data the moment the demo starts.
#
# Everything runs through the REAL endpoints — OTP, MyID KYC, the licence
# registry, escrow. Nothing is inserted straight into the database, so seeding
# is itself a full end-to-end test of the stack.
#
#   ./scripts/seed.sh [base-url]      (default http://localhost:8080)
set -uo pipefail

BASE="${1:-http://localhost:8080}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SELFIE="$(mktemp -t yukbor-selfie).jpg"
# A real (tiny) JPEG, so the multipart upload is genuine.
printf '\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00\xff\xd9' > "$SELFIE"
trap 'rm -f "$SELFIE"' EXIT

j() { python3 -c "import sys,json
try: d=json.load(sys.stdin)
except Exception: sys.exit(0)
for k in '$1'.split('.'):
    d = d[int(k)] if k.isdigit() else d.get(k, '')
    if d == '': break
print(d if d is not None else '')" 2>/dev/null; }

fail() { printf '  ERROR: %s\n' "$1" >&2; exit 1; }

# register <phone> <pinfl> <passport> <role> <fallback-name>
# Returns "id<TAB>accessToken". The name comes back from MyID, not from us.
register() {
  local phone=$1 pinfl=$2 passport=$3 role=$4 name=$5
  local vid code token out

  out=$(curl -sS -X POST "$BASE/auth/otp/request" -H 'Content-Type: application/json' \
        -d "{\"phoneNumber\":\"$phone\"}")
  vid=$(printf '%s' "$out" | j verificationId)
  code=$(printf '%s' "$out" | j devCode)
  [ -n "$vid" ] || fail "otp/request failed for $phone: $out"

  curl -sS -X POST "$BASE/auth/otp/verify" -H 'Content-Type: application/json' \
       -d "{\"verificationId\":\"$vid\",\"code\":\"$code\"}" >/dev/null

  out=$(curl -sS -X POST "$BASE/auth/myid/verify" \
        -F "verificationId=$vid" -F passportSeries=AB -F "passportNumber=$passport" \
        -F "pinfl=$pinfl" -F birthDate=1988-04-12 -F "selfie=@$SELFIE")
  token=$(printf '%s' "$out" | j myIdVerificationToken)
  [ -n "$token" ] || fail "myid/verify failed for $phone: $out"

  out=$(curl -sS -X POST "$BASE/auth/register" -H 'Content-Type: application/json' \
        -d "{\"fullName\":\"$name\",\"phoneNumber\":\"$phone\",\"role\":\"$role\",\"myIdVerificationToken\":\"$token\"}")
  printf '%s\t%s\t%s\n' "$(printf '%s' "$out" | j user.id)" \
                        "$(printf '%s' "$out" | j accessToken)" \
                        "$(printf '%s' "$out" | j user.fullName)"
}

order() { # <token> <json> -> order id
  curl -sS -X POST "$BASE/orders" -H "Authorization: Bearer $1" \
       -H 'Content-Type: application/json' -d "$2" | j id
}
accept() { # <token> <order> <leg> <executorId> <name>
  curl -sS -X POST "$BASE/orders/$2/accept" -H "Authorization: Bearer $1" \
       -H 'Content-Type: application/json' \
       -d "{\"leg\":\"$3\",\"executorId\":\"$4\",\"executorName\":\"$5\"}" >/dev/null
}
status() { # <token> <order> <leg> <status>
  curl -sS -X PATCH "$BASE/orders/$2/status" -H "Authorization: Bearer $1" \
       -H 'Content-Type: application/json' -d "{\"leg\":\"$3\",\"status\":\"$4\"}" >/dev/null
}

echo "seeding $BASE"
curl -fsS --max-time 5 "$BASE/health" >/dev/null || fail "gateway not reachable — run 'make up' first"

# ---- people -----------------------------------------------------------
# PINFL digits are load-bearing here (plan §10):
#   FIRST digit  = century + sex, so names come back correctly gendered
#   LAST  digit  = the licence bucket the simulated registry returns
#                  odd -> ["B"], 0/2/4 -> ["B","C"], 6/8 -> ["B","C","CE"]
echo "  registering users"
IFS=$'\t' read -r CLIENT_ID CLIENT_T CLIENT_NAME <<< "$(register '+998901112233' '42904123456782' '1810001' 'client' 'Клиент')"
IFS=$'\t' read -r DRV1_ID DRV1_T DRV1_NAME       <<< "$(register '+998901234567' '31905123456788' '1810002' 'driver' 'Водитель')"
IFS=$'\t' read -r DRV2_ID DRV2_T DRV2_NAME       <<< "$(register '+998977778899' '31812345678904' '1810003' 'driver' 'Водитель')"
IFS=$'\t' read -r EQ_ID   EQ_T   EQ_NAME         <<< "$(register '+998909876543' '32001234567896' '1810004' 'equipmentProvider' 'Спецтехника')"
IFS=$'\t' read -r LAB_ID  LAB_T  LAB_NAME        <<< "$(register '+998935554433' '32101234567890' '1810005' 'laborProvider' 'Бригада')"
IFS=$'\t' read -r ADMIN_ID ADMIN_T ADMIN_NAME    <<< "$(register '+998900000001' '31501234567896' '1810009' 'admin' 'Администратор')"

printf '    %-20s %s\n' client "$CLIENT_NAME"
printf '    %-20s %s  (B/C/CE)\n' driver "$DRV1_NAME"
printf '    %-20s %s  (B/C only)\n' driver "$DRV2_NAME"
printf '    %-20s %s\n' equipmentProvider "$EQ_NAME"
printf '    %-20s %s\n' laborProvider "$LAB_NAME"
printf '    %-20s %s\n' admin "$ADMIN_NAME"

# A driver who fails the licence check, so the dashboard shows a rejected
# applicant with a reason. Registration is EXPECTED to be refused here.
register '+998907654321' '31712345678901' '1810006' 'driver' 'Отклонён' >/dev/null 2>&1
printf '    %-20s %s\n' 'rejected driver' '+998907654321 (LICENSE_CATEGORY_MISMATCH)'

# ---- orders -----------------------------------------------------------
TASHKENT='"pickupAddress":"Ташкент, Юнусабадский р-н, ул. Амира Темура 15","pickupLocation":{"latitude":41.311081,"longitude":69.240562}'
SAMARKAND='"dropoffAddress":"Самарканд, ул. Регистан 5","dropoffLocation":{"latitude":39.654896,"longitude":66.959843}'

echo "  creating orders"

# 1. Combo order, fully completed and reviewed — the "finished business" row.
DONE=$(order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportWithOptions\",
 \"cargo\":{\"cargoType\":\"Стройматериалы\",\"weightTons\":12.5,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"flatbed\",\"specialInstructions\":\"Хрупкий груз\"},
 \"equipmentRequest\":{\"equipmentType\":\"crane\",\"durationHours\":4,\"notes\":\"Кран для разгрузки\"},
 \"laborRequest\":{\"workersCount\":3,\"durationHours\":2,\"taskDescription\":\"Погрузка/разгрузка\"},
 $TASHKENT,$SAMARKAND,\"scheduledDate\":\"2026-08-04T09:00:00Z\",\"currency\":\"UZS\",\"paymentMethod\":\"payme\"}")
accept "$DRV1_T" "$DONE" transport "$DRV1_ID" "$DRV1_NAME"
accept "$EQ_T"   "$DONE" equipment "$EQ_ID"   "$EQ_NAME"
accept "$LAB_T"  "$DONE" labor     "$LAB_ID"  "$LAB_NAME"
for s in loadingInProgress inTransit delivered; do status "$DRV1_T" "$DONE" transport "$s"; done
status "$EQ_T"  "$DONE" equipment delivered
status "$LAB_T" "$DONE" labor     delivered
curl -sS -X POST "$BASE/orders/$DONE/confirm-completion" -H "Authorization: Bearer $CLIENT_T" >/dev/null
for pair in "$DRV1_ID:5:Всё отлично, быстро и аккуратно" "$EQ_ID:5:Кран подали вовремя" "$LAB_ID:4:Работали хорошо"; do
  curl -sS -X POST "$BASE/reviews" -H "Authorization: Bearer $CLIENT_T" -H 'Content-Type: application/json' \
    -d "{\"orderId\":\"$DONE\",\"revieweeId\":\"${pair%%:*}\",\"rating\":$(echo "$pair" | cut -d: -f2),\"comment\":\"$(echo "$pair" | cut -d: -f3)\"}" >/dev/null
done
echo "    completed combo order, 3 payees released, 3 reviews"

# 2. In transit right now — the live row on the map.
LIVE=$(order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Мебель\",\"weightTons\":8,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"boxTruck\"},
 $TASHKENT,\"dropoffAddress\":\"Бухара, ул. Накшбанди 12\",\"dropoffLocation\":{\"latitude\":39.774900,\"longitude\":64.428600},
 \"scheduledDate\":\"2026-08-07T08:00:00Z\",\"currency\":\"UZS\",\"paymentMethod\":\"click\"}")
accept "$DRV1_T" "$LIVE" transport "$DRV1_ID" "$DRV1_NAME"
status "$DRV1_T" "$LIVE" transport loadingInProgress
status "$DRV1_T" "$LIVE" transport inTransit
echo "    order in transit (escrow held)"

# 3. Just accepted — money held, work not started.
ACC=$(order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Продукты (реф)\",\"weightTons\":6,\"requiresRefrigeration\":true,\"requiredVehicleType\":\"refrigerated\"},
 $TASHKENT,\"dropoffAddress\":\"Наманган, ул. Навоий 3\",\"dropoffLocation\":{\"latitude\":40.998300,\"longitude\":71.672600},
 \"scheduledDate\":\"2026-08-09T07:30:00Z\",\"currency\":\"UZS\",\"paymentMethod\":\"uzcard\"}")
accept "$DRV2_T" "$ACC" transport "$DRV2_ID" "$DRV2_NAME"
echo "    order accepted, awaiting pickup"

# 4-6. Open orders, one per leg type, so every executor feed has something.
order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Металлопрокат\",\"weightTons\":22,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"tractorTrailer\"},
 $TASHKENT,\"dropoffAddress\":\"Андижан, ул. Бабура 40\",\"dropoffLocation\":{\"latitude\":40.782100,\"longitude\":72.344200},
 \"scheduledDate\":\"2026-08-12T06:00:00Z\",\"currency\":\"UZS\"}" >/dev/null
order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"equipmentOnly\",
 \"equipmentRequest\":{\"equipmentType\":\"excavator\",\"durationHours\":6,\"notes\":\"Котлован под фундамент\"},
 \"pickupAddress\":\"Самарканд, ул. Дагбитская 9\",\"pickupLocation\":{\"latitude\":39.640000,\"longitude\":66.940000},
 \"dropoffAddress\":\"Самарканд, ул. Дагбитская 9\",\"dropoffLocation\":{\"latitude\":39.640000,\"longitude\":66.940000},
 \"scheduledDate\":\"2026-08-11T08:00:00Z\",\"currency\":\"UZS\"}" >/dev/null
order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"laborOnly\",
 \"laborRequest\":{\"workersCount\":5,\"durationHours\":8,\"taskDescription\":\"Разгрузка вагонов\"},
 \"pickupAddress\":\"Ташкент, Сергелийский р-н, склад 4\",\"pickupLocation\":{\"latitude\":41.220000,\"longitude\":69.220000},
 \"dropoffAddress\":\"Ташкент, Сергелийский р-н, склад 4\",\"dropoffLocation\":{\"latitude\":41.220000,\"longitude\":69.220000},
 \"scheduledDate\":\"2026-08-10T09:00:00Z\",\"currency\":\"UZS\"}" >/dev/null
echo "    3 open orders (transport / equipment / labor)"

# 7-8. Return loads waiting NEAR Samarkand, so the backhaul search — the
# differentiator feature — actually returns something when a driver finishes
# the Tashkent→Samarkand run above.
order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Керамическая плитка\",\"weightTons\":10,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"boxTruck\"},
 \"pickupAddress\":\"Самарканд, ул. Гагарина 88\",\"pickupLocation\":{\"latitude\":39.663000,\"longitude\":66.975000},
 \"dropoffAddress\":\"Ташкент, Чиланзар, кв. 12\",\"dropoffLocation\":{\"latitude\":41.285600,\"longitude\":69.203400},
 \"scheduledDate\":\"2026-08-08T14:00:00Z\",\"currency\":\"UZS\"}" >/dev/null
order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Хлопковое волокно\",\"weightTons\":15,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"flatbed\"},
 \"pickupAddress\":\"Самарканд, Ургутский р-н\",\"pickupLocation\":{\"latitude\":39.700000,\"longitude\":67.010000},
 \"dropoffAddress\":\"Ташкент, Яшнабад\",\"dropoffLocation\":{\"latitude\":41.290000,\"longitude\":69.330000},
 \"scheduledDate\":\"2026-08-08T16:00:00Z\",\"currency\":\"UZS\"}" >/dev/null
echo "    2 return loads near Samarkand (backhaul demo)"

# 9. A cancelled order, so the dashboard is not uniformly happy.
CAN=$(order "$CLIENT_T" "{\"clientName\":\"$CLIENT_NAME\",\"type\":\"transportOnly\",
 \"cargo\":{\"cargoType\":\"Оборудование\",\"weightTons\":4,\"requiresRefrigeration\":false,\"requiredVehicleType\":\"boxTruck\"},
 $TASHKENT,$SAMARKAND,\"scheduledDate\":\"2026-08-15T09:00:00Z\",\"currency\":\"UZS\"}")
curl -sS -X POST "$BASE/orders/$CAN/cancel" -H "Authorization: Bearer $CLIENT_T" >/dev/null
echo "    1 cancelled order"

# ---- hand the dashboard its token --------------------------------------
# Local development only. In production the dashboard is built with NO token
# (a Vite VITE_* value is inlined into a public bundle) and Caddy injects the
# Authorization header server-side instead — see docs/DEPLOY.md.
printf 'VITE_USE_MOCKS=false\nVITE_ADMIN_TOKEN=%s\n' "$ADMIN_T" > "$REPO/dashboard/.env.local"

# Ids and tokens for scripting — used by seed-prod.sh to mint a long-lived
# admin token, since the one issued above expires in 24h.
umask 077
cat > "$REPO/.seed-tokens" <<EOF
CLIENT_ID=$CLIENT_ID
DRIVER_ID=$DRV1_ID
ADMIN_ID=$ADMIN_ID
CLIENT_TOKEN=$CLIENT_T
DRIVER_TOKEN=$DRV1_T
ADMIN_TOKEN=$ADMIN_T
EOF

echo
echo "done. dashboard/.env.local written — 'make dashboard' now shows live data."
echo
curl -sS "$BASE/admin/stats" -H "Authorization: Bearer $ADMIN_T" | python3 -m json.tool | sed 's/^/  /'
echo
echo "  tokens (export these to drive the API by hand):"
echo "    export CLIENT_TOKEN=$CLIENT_T"
echo "    export DRIVER_TOKEN=$DRV1_T"
echo "    export ADMIN_TOKEN=$ADMIN_T"
