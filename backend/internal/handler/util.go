package handler

// asInt64 将 interface{} 安全转换为 int64，支持 int/int64/float64。
func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

// scopeFromRoles 根据角色编码推断数据范围权限。
func scopeFromRoles(codes []string) string {
	for _, r := range codes {
		if r == "super_admin" {
			return "all"
		}
	}
	for _, r := range codes {
		if r == "dept_admin" {
			return "dept"
		}
	}
	return "self"
}
