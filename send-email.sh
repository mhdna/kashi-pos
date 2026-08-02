#!/usr/bin/env bash

set -euo pipefail

usage() {
    echo "Usage: $0 <csv-file> [recipient-email] [subject] [sender-email]" >&2
    echo "       TARGET_EMAIL can be used instead of the recipient argument." >&2
    echo "       SENDER_EMAIL can be used instead of the sender argument." >&2
    exit 2
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" || $# -lt 1 ]]; then
    usage
fi

csv_path="$1"
target_email="${2:-${TARGET_EMAIL:-}}"
email_date="${EMAIL_DATE:-$(date +%Y-%m-%d)}"
email_branch="${EMAIL_BRANCH:-Kashi}"
subject="${3:-${email_branch} Sales Report}"
sender_email="${4:-${SENDER_EMAIL:-}}"

if [[ ! -f "$csv_path" ]]; then
    echo "CSV file not found: $csv_path" >&2
    exit 1
fi
if [[ -z "$target_email" ]]; then
    echo "Recipient email is required." >&2
    usage
fi

abs_csv_path="$(cd "$(dirname "$csv_path")" && pwd)/$(basename "$csv_path")"

if ! output="$(osascript - "$abs_csv_path" "$target_email" "$subject" "$sender_email" <<'APPLESCRIPT'
on run argv
    set csvPath to item 1 of argv
    set targetEmail to item 2 of argv
    set subjectLine to item 3 of argv
    set senderEmail to item 4 of argv

    tell application "Microsoft Outlook"
        set newMessage to make new outgoing message with properties {subject:subjectLine, content:"Please find the attached sales report."}
        if senderEmail is not "" then
            try
                set account of newMessage to senderEmail
            end try
        end if
        tell newMessage
            make new recipient at newMessage with properties {email address:{address:targetEmail}}
            make new attachment with properties {file:POSIX file csvPath}
            open
        end tell
    end tell
end run
APPLESCRIPT
)"; then
    echo "Failed to create Outlook draft:" >&2
    echo "$output" >&2
    exit 1
fi

echo "Email draft created in Microsoft Outlook."
echo "Attachment: $abs_csv_path"
echo "To: $target_email"
echo "Subject: $subject"
