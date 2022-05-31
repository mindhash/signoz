import { LockOutlined } from '@ant-design/icons';
import { Button, Typography } from 'antd';
import UpgradeModal from 'components/Upgrade';
import React, { useState } from 'react';

import { TitleWrapper } from './styles';

function Authentication(): JSX.Element {
	const [upgradeVisible, setUpgradeVisible] = useState<boolean>(true);

	const renderMethod = (): JSX.Element => {
		return (
			<>
				<Typography.Title level={5}>Google Apps authentication </Typography.Title>
				<Typography.Paragraph>
					Let your team sign in with a Google account.
				</Typography.Paragraph>

				<Button icon={<LockOutlined />} type="primary">
					Upgrade to Configure
				</Button>

				<Typography.Title level={5} style={{ marginTop: '1rem' }}>
					SAML authentication{' '}
				</Typography.Title>
				<Typography.Paragraph>
					Setup Azure, Okta, OneLogin or your custom SAML 2.0 provider.
				</Typography.Paragraph>
				<Button icon={<LockOutlined />} type="primary">
					Upgrade to Configure
				</Button>
			</>
		);
	};
	return (
		<>
			<UpgradeModal visible={upgradeVisible} setVisible={setUpgradeVisible} />
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

export default Authentication;
