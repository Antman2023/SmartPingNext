package http

// Seek to each preceding date using the existing date(logtime) index rather
// than visiting every alert entry. The NULL branch preserves the API's scan
// error for malformed timestamps instead of silently hiding corrupt records.
const alertDatesQuery = `WITH RECURSIVE alert_dates(ldate) AS (
	SELECT max(date(logtime)) FROM alertlog
	UNION ALL
	SELECT (SELECT max(date(logtime)) FROM alertlog WHERE date(logtime) < alert_dates.ldate)
	FROM alert_dates WHERE ldate IS NOT NULL
)
SELECT ldate FROM alert_dates WHERE ldate IS NOT NULL
UNION ALL
SELECT NULL WHERE EXISTS (SELECT 1 FROM alertlog WHERE date(logtime) IS NULL)
ORDER BY ldate DESC`
