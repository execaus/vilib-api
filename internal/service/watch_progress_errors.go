package service

// ErrIntervalInvalid — присланный клиентом интервал heartbeat'а некорректен: to_ms < from_ms
// либо превышает известную длительность видео (§3 дизайна эпика Э3).
var ErrIntervalInvalid = NewValidationErrorCode("validation.interval", "invalid heartbeat interval")
