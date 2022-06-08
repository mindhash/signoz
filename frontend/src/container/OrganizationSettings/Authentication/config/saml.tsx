import React from 'react';
import { LockOutlined, ToolOutlined } from '@ant-design/icons';
import { Button, Typography } from 'antd';
import Spinner from 'components/Spinner';
import { useQueries } from 'react-query';

import getFeatureFlags from 'api/user/getFeatureFlags';
import { ModalType, SAMLConfig } from 'container/OrganizationSettings/types';

// SamlAuthConfig displays SAML section and allows editing of parameters
// through SamlParamsForm. If user is not in a paid plan, an option
// to switch to a paid plan is shown.
function SamlAuthSection({
	showModal,
	setShowModal,
}: SamlSectionProps): JSX.Element {
	// get or set saml params 
	// SAMLParams
	// add a refresh button to bring back the exisitng params ?
	const [getFeatures] = useQueries([
		{
			queryFn: getFeatureFlags,
			queryKey: 'getFeatureFlags',
			enabled: true,
		},
	]);

	const renderAction = (): JSX.Element => {
		if (getFeatures.isLoading) {
			return <Spinner tip="Loading..." />;
		}

		if (getFeatures.isSuccess && getFeatures.data?.statusCode === 200) {
			const { features } = getFeatures.data.payload;
			if (features && features.SAML) {
				return (
					<Button
						icon={<ToolOutlined />}
						type="primary"
						onClick={(e): void => {
							e.preventDefault();
							setShowModal('EDIT_SAML');
						}}
					>
						{' '}
						Edit Settings{' '}
					</Button>
				);
			}

			return (
				<Button
					icon={<LockOutlined />}
					type="primary"
					onClick={(e): void => {
						e.preventDefault();
						setShowModal('APPLY_LICENSE');
					}}
				>
					Switch to a Paid Plan
				</Button>
			);
		}
		return <div />;
	};

	return (
		<>
			{showModal? <SamlParamsForm />}
			<Typography.Title level={5} style={{ marginTop: '1rem' }}>
				SAML authentication{' '}
			</Typography.Title>
			<Typography.Paragraph>
				Setup Azure, Okta, OneLogin or your custom SAML 2.0 provider.
			</Typography.Paragraph>
			{renderAction()}
		</>
	);
}

interface SamlSectionProps {
	showModal: ModalType;
	setShowModal: (m: ModalType) => void;
}

export default SamlAuthSection;

// SamlConfigForm allows editing of SAML parameters. The content
// is displayed in a modal window. User can only edit parameters
// for the org she is associated with.
function SamlParamsForm(
{	setParams,
	saveParams,
	hideForm,
}: SamlParamsFormProps 
): JSX.Element {}


interface SamlParamsFormProps {
	setParams: (p: SAMLParams) => void,
	saveParams: (p: SAMLParams) => void,
	hideForm: () => void,
}