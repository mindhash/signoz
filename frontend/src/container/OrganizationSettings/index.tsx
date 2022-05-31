import { Divider, Space } from 'antd';
import useEEAvailable from 'hooks/useEEAvailable';
import React from 'react';
import { useSelector } from 'react-redux';
import { AppState } from 'store/reducers';
import AppReducer from 'types/reducer/app';

import Authentication from './Authentication';
import DisplayName from './DisplayName';
import Members from './Members';
import PendingInvitesContainer from './PendingInvitesContainer';

function OrganizationSettings(): JSX.Element {
	const ee = useEEAvailable();
	const { org } = useSelector<AppState, AppReducer>((state) => state.app);

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
			{ee && (
				<>
					<Authentication />
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
