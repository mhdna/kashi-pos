# WhatsApp restriction: what happened and what to do

Written 2026-10-26, right after the Centro sender number was restricted. Read this before
resuming the sender, or before sending anything new through WhatsApp.

## What happened

- The Centro sender number was in normal use: invoice polls to customers, about **28 messages a
  day**, running fine for a long time with no problems.
- On the day before this note, we sent a **Buy 2 Get 1 (B2G1) promo to about 40 customers**
  through the same automated sender, while the polls were still going out at the same time.
- That day the sender sent **79 messages** (usual 28.2), nearly 3x normal, with ~40 of them
  promotional and sent close together.
- About 18 hours before this note, the linked device was **logged out by WhatsApp**
  (`loggedOut`, code 401). The dashboard paused sending automatically and said to check the
  phone for a ban notice.
- On the phone, WhatsApp showed: **"Your account is restricted. You can't start new chats right
  now."** The details screen had a countdown (about 6 h left when we looked) and this reason:
  *"Recent activity on your account may not have followed WhatsApp terms on automated or bulk
  messaging."* It pointed to "official WhatsApp Business tools".

Other numbers from the dashboard that day: 2 disconnects, 0 refused by WhatsApp, 0 failed
sends, 1 stuck on one tick. So WhatsApp did not refuse any single send. It flagged the
pattern, then killed the session.

## Diagnosis

- The **promo blast was the trigger, not the polls.** Polls ran fine for a long time at normal
  volume. The restriction came only after the B2G1 promo.
- Likely reasons the promo hurt more than the polls:
  - Promotions look like marketing and are policed harder than transactional messages.
  - Some of the ~40 recipients probably didn't expect it. Blocks and "report" taps are the
    biggest trigger for restrictions.
  - The promo and the polls went out together, so volume jumped to ~3x in one day.
- This is a **temporary restriction, not a ban**, but the number is now flagged. A second
  strike is treated harder: longer restriction, then a permanent ban.
- The `loggedOut` 401 was WhatsApp enforcing the restriction. It was not a bug in our code.

## Do NOT

- **Do not resume sending the moment the countdown ends.** Sending through a linked device
  right after a restriction is the usual way a temporary restriction becomes a permanent ban.
- **Do not send promos or marketing through the automated sender.** B2G1 and similar deals are
  exactly what got this number flagged.
- **Do not dump the queued backlog** in one go after resuming.
- **Do not send to cold contacts** (people who haven't chatted with the number or saved it).
- **Do not send the same text to many people in a burst.** Identical bulk messages look
  automated.
- **Do not go much above the normal daily volume** (~28/day). A jump to 79 was a sudden 3x.
- **Do not keep relinking and retrying** if it logs out again. Stop and reassess.
- **Do not spam "Request review"** in the app. Use it once at most.

## Do

1. **Wait out the restriction completely**, then wait **a few more quiet days** (2-3 minimum,
   more is better) before any automated sending. Normal manual use of WhatsApp is fine
   meanwhile. Replying to people who message first is allowed during the restriction.
2. **Check the phone before relinking.** If there is any ban notice or verify-number prompt,
   stop. Only relink if WhatsApp opens normally.
3. **Resume only the polls**, and only to **customers with invoices of $50 or more**. That
   cuts volume a lot and picks customers who just paid and are least likely to report spam.
4. **Ramp up slowly**: about 5-10 messages a day at first, then raise it over a week or two
   up to the old ~28/day. Never above.
5. **Prefer people who know the number**: customers who have chatted with it or saved it.
6. **Vary the wording and the timing** so messages don't go out in identical bursts.
7. **Stop messaging anyone** who blocks or complains.
8. **Polls are a bit riskier than plain text** (a distinct bulk-looking format). A normal
   one-line message with a link, or "reply 1-5", is a safer substitute if trouble returns.
9. **Run promos by hand** in small batches to customers who know the shop, or use the official
   WhatsApp Business tools / Cloud API.

## If it happens again

- A second logout or restriction: **stop all automated sending**. Do not relink.
- Expect a longer restriction, then a possible permanent ban.
- Fall back to a different number only as a stopgap. The same thing can happen to it.

## Longer-term fix

The only stable option is the official **WhatsApp Business Platform (Cloud API)**: message
templates, a small per-conversation fee, and no ban risk for automation. Worth moving to if
this number matters to the business.

## Ideas for the sender (not built yet unless noted in git history)

- Hard **daily cap** enforced in code.
- A **ramp-up schedule** after any pause or relink.
- A **minimum invoice amount** filter (the $50+ rule).
- A **previous contact only** filter.
- **Block promo-type messages** from going through the automated sender.
- Randomised delays between sends.

These lower the risk. None of them remove it.
