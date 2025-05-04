package generated

// GetPrimaryMediaKey safely accesses the first mediaKey
func (x *RemoteMatches) GetMediaKey() string {
	if x == nil || x.Field1 == nil || x.Field1.Field2 == nil || x.Field1.Field2.Field2 == nil {
		return ""
	}
	return x.Field1.Field2.Field2.MediaKey
}
