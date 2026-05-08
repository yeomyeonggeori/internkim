import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { isNodeRequest, normalizeFleetID } from '$lib/device-auth';
import { kv } from '$lib/kv';
import type { Device, FleetMember } from '$lib/types';
import { activeFleetMembers, fleetQuorumSize, pendingFleetMembers } from '$lib/fleet';

const corsHeaders = {
	'Access-Control-Allow-Origin': '*',
	'Access-Control-Allow-Methods': 'GET, OPTIONS',
	'Access-Control-Allow-Headers': 'Content-Type, X-InternKim-Fleet-ID, X-InternKim-Fleet-Secret'
};

export const OPTIONS: RequestHandler = async () => {
	return new Response(null, { headers: corsHeaders });
};

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const env = platform?.env;
	if (!env?.KV) throw error(500, 'KV not available');

	const fleetID = normalizeFleetID(url.searchParams.get('fleet_id') ?? '');
	if (!fleetID) throw error(400, 'fleet_id required');

	const device = await kv.getDevice(env.KV, fleetID);
	if (!device) throw error(404, 'Fleet not found');

	const adminToken = url.searchParams.get('admin_token') ?? '';
	const isAuthorizedNode = await isNodeRequest(request, device, fleetID);
	if (!isAuthorizedNode && adminToken !== env.INTERNKIM_REGISTER_SECRET) {
		throw error(403, 'Fleet metadata requires node auth');
	}

	return json(fleetMetadataResponse(device), { headers: corsHeaders });
};

function fleetMetadataResponse(device: Device) {
	const fleet = device.fleet ?? {
		fleetID: device.fleet_id,
		members: []
	};
	const activeNodes = activeFleetMembers(fleet);
	const pendingNodes = pendingFleetMembers(fleet);

	return {
		fleet_id: device.fleet_id,
		default_node_id: activeNodes[0]?.nodeID ?? '',
		quorum_size: fleetQuorumSize(fleet),
		active_nodes: activeNodes.map(fleetMemberResponse),
		pending_nodes: pendingNodes.map(fleetMemberResponse)
	};
}

function fleetMemberResponse(member: FleetMember) {
	return {
		node_id: member.nodeID,
		status: member.status,
		ssh_hostname: member.sshHostname ?? '',
		joined_at: member.joinedAt,
		activated_at: member.activatedAt ?? ''
	};
}
