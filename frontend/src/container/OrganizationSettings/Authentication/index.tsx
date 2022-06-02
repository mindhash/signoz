import { Typography } from 'antd';
import LicenseModal from 'components/License';
import { ModalType } from 'container/OrganizationSettings/types';
import React from 'react';

import SamlAuthSection from './config/saml';
import { TitleWrapper } from './styles';

function AuthSettings({ showModal, setShowModal }: Props): JSX.Element {
	const renderMethod = (): JSX.Element => {
		return <SamlAuthSection showModal={showModal} setShowModal={setShowModal} />;
	};

	const hideLicenseModal = (b: boolean): void => {
		if (!b) {
			setShowModal('');
		}
	};

	const renderLicenseModal = (): JSX.Element => {
		return (
			<LicenseModal
				visible={showModal === 'APPLY_LICENSE'}
				setVisible={hideLicenseModal}
			/>
		);
	};

	return (
		<>
			{renderLicenseModal()}
			<TitleWrapper>
				<Typography.Title level={3}>Authentication</Typography.Title>
			</TitleWrapper>
			<p>
				SigNoz supports a number of single sign-on (SSO) services. Get started with
				setting up your SSO below or <a href="/">learn more.</a>
			</p>
			{renderMethod()}
		</>
	);
}

interface Props {
	showModal: ModalType;
	setShowModal: (m: ModalType) => void;
}

export default AuthSettings;
