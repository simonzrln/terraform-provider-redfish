# The ID is a JSON object with the connection details; add "system_id" if the BMC has more than one system.
terraform import redfish_bios.board '{"redfish_alias":"bmc"}'
