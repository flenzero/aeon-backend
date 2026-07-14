#!/usr/bin/env bash
set -euo pipefail

echo "== account-api =="
curl -i https://api.aeonblight.com/health
echo
curl -i http://api.aeonblight.com/ready
echo

echo "== economy-api =="
curl -i https://api.aeonblight.com/api/economy/health
echo
curl -i https://api.aeonblight.com/api/economy/ready
echo

echo "== admin-api =="
curl -i https://api.aeonblight.com/api/admin/health
echo
curl -i https://api.aeonblight.com/api/admin/ready
echo

echo "== public auth checks =="
curl -i "https://api.aeonblight.com/api/auth/wallet/nonce?walletAddress=7V7VS1mQg2ZsqGD5SPjVkQDe3cL8AmHHcTGjbtXDNTsr"
echo
curl -i "https://api.aeonblight.com/api/admin/auth/nonce?adminId=ops-01"
echo

echo "== super admin check =="
curl -i "https://api.aeonblight.com/api/admin/admin-users" \
  -H "X-Super-Admin-Key: aeonops26"
echo