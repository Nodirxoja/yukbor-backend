package httpx

// Error codes. The exact strings the iOS app branches on live here so the
// pre-demo audit is a single-file read against docs/API_CONTRACT.md §9.
// Never change one of these without changing the contract first.
const (
	// Auth / OTP (contract §1)
	CodeOTPInvalid             = "OTP_INVALID"
	CodeOTPExpired             = "OTP_EXPIRED"
	CodeOTPRateLimited         = "OTP_RATE_LIMITED"
	CodeOTPNotVerified         = "OTP_NOT_VERIFIED"
	CodePhoneAlreadyRegistered = "PHONE_ALREADY_REGISTERED"
	CodeUserNotFound           = "USER_NOT_FOUND"

	// MyID KYC (contract §1, v1.1)
	CodePassportNotFound = "PASSPORT_NOT_FOUND"
	CodeFaceMismatch     = "FACE_MISMATCH"
	CodeMyIDUnavailable  = "MYID_SERVICE_UNAVAILABLE"
	CodeMyIDTokenInvalid = "MYID_TOKEN_EXPIRED_OR_INVALID"

	// Driver licence registry (plan §5/§10)
	CodeLicenseCategoryMismatch = "LICENSE_CATEGORY_MISMATCH"

	// Orders (contract §3)
	CodeOrderNotFound           = "ORDER_NOT_FOUND"
	CodeOrderNotPublished       = "ORDER_NOT_PUBLISHED"
	CodeLegAlreadyTaken         = "LEG_ALREADY_TAKEN"
	CodeLegNotFound             = "LEG_NOT_FOUND"
	CodeInvalidStatusTransition = "INVALID_STATUS_TRANSITION"
	CodeOrderNotReady           = "ORDER_NOT_READY"
	CodeOrderNotCancellable     = "ORDER_NOT_CANCELLABLE"

	// Wallet (contract §4)
	CodePaymentDeclined     = "PAYMENT_DECLINED"
	CodeTransactionNotFound = "TRANSACTION_NOT_FOUND"

	// Reviews (contract §6)
	CodeReviewAlreadyExists = "REVIEW_ALREADY_EXISTS"
	CodeOrderNotCompleted   = "ORDER_NOT_COMPLETED"

	// Generic
	CodeBadRequest   = "BAD_REQUEST"
	CodeValidation   = "VALIDATION_ERROR"
	CodeUnauthorized = "UNAUTHORIZED"
	CodeForbidden    = "FORBIDDEN"
	CodeNotFound     = "NOT_FOUND"
	CodeInternal     = "INTERNAL_ERROR"
)
