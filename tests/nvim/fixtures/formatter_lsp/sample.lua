local function describe_user(name, roles)
local summary={name=name,roles=roles or {}}
return summary
end

return describe_user("luis",{"editor","reviewer"})
