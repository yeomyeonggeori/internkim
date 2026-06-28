# Banchan Table Reservation Prototype Source Data

## Brand

- Brand name: Banchan Table
- Audience: busy single-person households in Seoul
- Promise: reserve a Korean meal kit by Thursday and pick it up fresh on the weekend
- Prototype purpose: test whether users understand the reservation flow without explanation

## Reservation Rules

- Available pickup dates: 2026-07-18, 2026-07-19, 2026-07-25
- Pickup windows: 11:00-13:00, 17:00-19:00
- Reservation cutoff: Thursday 20:00 before the selected pickup weekend
- Pickup location: Seongsu Community Kitchen
- Payment in prototype: pay at pickup

## Menu Options

| Menu ID | Name | Servings | Price KRW | Prep Time | Tags |
|---|---|---:|---:|---|---|
| soy-bulgogi | Soy Bulgogi Set | 2 | 16900 | 12 minutes | protein-rich, mild |
| doenjang-stew | Doenjang Stew Kit | 2 | 13900 | 15 minutes | comforting, vegetarian option |
| spicy-pork | Spicy Pork Rice Bowl Kit | 2 | 14900 | 10 minutes | spicy, quick |
| mushroom-jeon | Mushroom Jeon Plate | 1 | 11900 | 8 minutes | vegetarian, light |

## Order Summary Rules

- Quantity choices: 1, 2, or 3 kits per menu item
- Total price is menu price multiplied by quantity.
- Show pickup date, pickup window, selected menu, quantity, and total price.
- Confirmation state text: "Reservation held - pay at pickup"

## Required Interface

- First screen must show the reservation experience, not a marketing-only hero.
- User must be able to choose a menu, pickup date, pickup window, and quantity.
- User must see a live order summary.
- User must see a confirmation state after pressing the reservation button.
- Mobile layout must keep controls readable without horizontal scrolling.

## Constraints

- Do not invent additional menus, dates, discounts, delivery options, reviews, addresses, phone numbers, or payment methods.
- If a requested value is missing, display "Not provided" rather than inventing a value.
