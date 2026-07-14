#!/usr/bin/env python3
"""Generate dungeons.json + dungeon loot pools from BNBLAND floors.json."""

from __future__ import annotations

import json
import math
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SRC = Path("/Users/TJ/code/Project/BNBLAND/bnb-game/ECONOMY/export/floors.json")
DUNGEONS_OUT = ROOT / "configs/economy/dungeons.json"
LOOT_OUT = ROOT / "configs/economy/loot_pools.json"

CHAPTER_META = {
    0: {"name": "Ashen Threshold", "ticket": "boss_ticket_ashen_threshold"},
    1: {"name": "Gloomwood", "ticket": "boss_ticket_gloomwood"},
    2: {"name": "Voidscar", "ticket": "boss_ticket_voidscar"},
}

STAGE_PREFIX = {
    1: "ashbound",
    5: "gloomhide",
    10: "shadowiron",
    15: "nightglass",
    20: "eclipseguard",
    25: "voidforged",
    30: "aeonblight",
}

# old item prefix/id -> (seriesId, is_weapon)
SERIES_MAP = {
    "wpn_sword": ("sword", True),
    "wpn_bow": ("bow", True),
    "wpn_dagger": ("axe", True),
    "wpn_staff": ("staff", True),
    "helmet": ("helmet", False),
    "armor": ("chest", False),
    "cloth": ("gloves", False),
    "pant": ("shoes", False),
    "cape": ("cloak", False),
    "accessory": ("accessory", False),
}

MATERIAL_MAP = {
    "wood": "ashwood_white",
    "gold_ore": "shadow_iron",
}

PASS_MAP = {
    "pass_boss_0": "boss_ticket_ashen_threshold",
    "pass_boss_1": "boss_ticket_gloomwood",
    "pass_boss_2": "boss_ticket_voidscar",
}

# floor -> shared loot pool id
POOL_BY_FLOOR = {
    **{f: "dungeon_ch0_f1_3" for f in (1, 2, 3)},
    **{f: "dungeon_ch0_f4_6" for f in (4, 5, 6)},
    **{f: "dungeon_ch0_f7_9" for f in (7, 8, 9)},
    10: "dungeon_ch0_boss",
    **{f: "dungeon_ch1_f11_13" for f in (11, 12, 13)},
    **{f: "dungeon_ch1_f14_16" for f in (14, 15, 16)},
    **{f: "dungeon_ch1_f17_19" for f in (17, 18, 19)},
    20: "dungeon_ch1_boss",
    **{f: "dungeon_ch2_f21_23" for f in (21, 22, 23)},
    **{f: "dungeon_ch2_f24_26" for f in (24, 25, 26)},
    **{f: "dungeon_ch2_f27_29" for f in (27, 28, 29)},
    30: "dungeon_ch2_boss",
}

# representative floor used to copy loot drops for each shared pool
POOL_SOURCE_FLOOR = {
    "dungeon_ch0_f1_3": 1,
    "dungeon_ch0_f4_6": 4,
    "dungeon_ch0_f7_9": 7,
    "dungeon_ch0_boss": 10,
    "dungeon_ch1_f11_13": 11,
    "dungeon_ch1_f14_16": 14,
    "dungeon_ch1_f17_19": 17,
    "dungeon_ch1_boss": 20,
    "dungeon_ch2_f21_23": 21,
    "dungeon_ch2_f24_26": 24,
    "dungeon_ch2_f27_29": 27,
    "dungeon_ch2_boss": 30,
}


def max_exp(floor_id: int, is_boss: bool) -> int:
    base = 20 + floor_id * 10
    if is_boss:
        return int(math.floor(base * 1.5))
    return base


def map_equipment(item_id: str) -> tuple[str, str] | None:
    m = re.fullmatch(r"(.+)_l(\d+)", item_id)
    if not m:
        return None
    prefix, stage_s = m.group(1), int(m.group(2))
    if prefix not in SERIES_MAP or stage_s not in STAGE_PREFIX:
        raise KeyError(f"unmapped equipment {item_id}")
    series, is_weapon = SERIES_MAP[prefix]
    new_id = f"{STAGE_PREFIX[stage_s]}_{series}_t{stage_s}"
    affix = "weapon_affixes" if is_weapon else "non_weapon_affixes"
    return new_id, affix


def rarity_for(floor_id: int, item_id: str, is_boss: bool) -> int:
    if not is_boss:
        return 1
    # floor 30 t30 pieces are rarity 3; t25 stay rarity 2
    if floor_id == 30 and item_id.endswith("_t30"):
        return 3
    return 2


def convert_loot_drop(drop: dict, floor_id: int, is_boss: bool) -> dict:
    item_id = drop["itemId"]
    quantity = int(drop["quantity"])
    chance = round(float(drop["dropChance"]), 4)

    if item_id in PASS_MAP:
        return {
            "rewardType": "item",
            "itemId": PASS_MAP[item_id],
            "quantityMin": quantity,
            "quantityMax": quantity,
            "dropChance": chance,
        }
    if item_id in MATERIAL_MAP:
        return {
            "rewardType": "item",
            "itemId": MATERIAL_MAP[item_id],
            "quantityMin": quantity,
            "quantityMax": quantity,
            "dropChance": chance,
        }
    mapped = map_equipment(item_id)
    if mapped is None:
        raise KeyError(f"unknown loot item {item_id}")
    new_id, affix = mapped
    return {
        "rewardType": "equipment",
        "itemId": new_id,
        "quantityMin": 1,
        "quantityMax": 1,
        "dropChance": chance,
        "rarity": rarity_for(floor_id, new_id, is_boss),
        "affixPoolId": affix,
    }


def main() -> None:
    src = json.loads(SRC.read_text())
    floors_by_id: dict[int, dict] = {}
    chapters_out = []

    for chapter in src["chapters"]:
        chapter_id = chapter["chapterId"]
        meta = CHAPTER_META[chapter_id]
        floors_out = []
        for floor in chapter["floors"]:
            floor_id = floor["floorId"]
            is_boss = bool(floor.get("requiredItemId"))
            floors_by_id[floor_id] = {**floor, "_chapter": chapter_id, "_isBoss": is_boss}

            enter_items = []
            if is_boss:
                enter_items.append(
                    {"itemId": meta["ticket"], "quantity": 1}
                )

            floors_out.append(
                {
                    "floorId": floor_id,
                    "isBoss": is_boss,
                    "enterCost": {"gold": int(floor["goldCost"]), "items": enter_items},
                    "maxExp": max_exp(floor_id, is_boss),
                    "lootPoolId": POOL_BY_FLOOR[floor_id],
                    "enemyHpScale": float(floor["enemyHpScale"]),
                    "enemyAtkScale": float(floor["enemyAtkScale"]),
                }
            )
        chapters_out.append(
            {
                "chapterId": chapter_id,
                "name": meta["name"],
                "floors": floors_out,
            }
        )

    DUNGEONS_OUT.write_text(json.dumps({"chapters": chapters_out}, indent=2) + "\n")

    existing = json.loads(LOOT_OUT.read_text())
    keep = [
        pool
        for pool in existing["lootPools"]
        if not str(pool["lootPoolId"]).startswith("dungeon_ch")
    ]

    dungeon_pools = []
    for pool_id, source_floor in POOL_SOURCE_FLOOR.items():
        floor = floors_by_id[source_floor]
        entries = [
            convert_loot_drop(drop, source_floor, floor["_isBoss"])
            for drop in floor["lootDrops"]
        ]
        dungeon_pools.append({"lootPoolId": pool_id, "entries": entries})

    LOOT_OUT.write_text(
        json.dumps({"lootPools": dungeon_pools + keep}, indent=2) + "\n"
    )
    print(f"wrote {DUNGEONS_OUT}")
    print(f"wrote {LOOT_OUT} ({len(dungeon_pools)} dungeon pools + {len(keep)} other)")


if __name__ == "__main__":
    main()
