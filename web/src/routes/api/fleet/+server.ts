import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { isBoardRequest, normalizeDeviceID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import type { Device, FleetMember } from '$lib/types';
import { activeFleetMembers, fleetQuorumSize, pendingFleetMembers } from '$lib/fleet';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-InternKim-Device-ID, X-InternKim-Device-Secret'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const deviceID = normalizeDeviceID(url.searchParams.get('device_id') ?? '');
	if (!deviceID) throw error(400, 'device_id required');

	const device = await kv.getDevice(env.KV, deviceID);
	if (!device) throw error(404, 'Device not found');

	const adminToken = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedBoard = await isBoardRequest(request, device, deviceID);
	if (!isAuthorizedBoard && adminToken !== env.INTERNKIM_REGISTER_SECRET) {
		throw error(403, 'Fleet metadata requires board auth');
	}

	return json(fleetMetadataResponse(device), { headers: corsHeaders });
};

function fleetMetadataResponse(device: Device) {
	const fleet = device.fleet ?? {
		fleetID: device.device_id,
		members: []
	};
	const activeNodes = activeFleetMembers(fleet);
	const pendingNodes = pendingFleetMembers(fleet);

	return {
		device_id: device.device_id,
		default_node_id: activeNodes[0]?.boardID ?? '',
		quorum_size: fleetQuorumSize(fleet),
		active_nodes: activeNodes.map(fleetMemberResponse),
		pending_nodes: pendingNodes.map(fleetMemberResponse)
	};
}

function fleetMemberResponse(member: FleetMember) {
	return {
		node_id: member.boardID,
		status: member.status,
		ssh_hostname: member.sshHostname ?? '',
		joined_at: member.joinedAt,
		activated_at: member.activatedAt ?? ''
	};
}
