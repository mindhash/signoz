import { Divider, Space } from 'antd';
import getUserVersion from 'api/user/getVersion';
import React, { useState } from 'react';
import { useQueries } from 'react-query';
import { useSelector } from 'react-redux';
import { AppState } from 'store/reducers';
import AppReducer from 'types/reducer/app';

import { MODAL_TYPE } from './types';
import Authentication from './Authentication';
import DisplayName from './DisplayName';
import Members from './Members';
import PendingInvitesContainer from './PendingInvitesContainer';

function OrganizationSettings(): JSX.Element {
	// a page level modal state manager to make sure only
	// one modal is displayed at time
	const [showModal, setShowModal] = useState<MODAL_TYPE>('');
	const { org, isLoggedIn } = useSelector<AppState, AppReducer>(
		(state) => state.app,
	);

	const [versionResponse] = useQueries([
		{
			queryFn: getUserVersion,
			queryKey: 'getEEAvailable',
			enabled: isLoggedIn,
		},
	]);

	if (!org) {
		return <div />;
	}

	return (
		<>
			<Space direction="vertical">
				{org.map((e, index) => (
					<DisplayName
						isAnonymous={e.isAnonymous}
						key={e.id}
						id={e.id}
						index={index}
					/>
				))}
			</Space>
			<Divider />
			{versionResponse && versionResponse.data?.payload?.eeAvailable && (
				<>
					<Authentication showModal={showModal} setshowModal={showModal} />
					<Divider />
				</>
			)}
			<PendingInvitesContainer />
			<Divider />
			<Members />
		</>
	);
}

export default OrganizationSettings;
