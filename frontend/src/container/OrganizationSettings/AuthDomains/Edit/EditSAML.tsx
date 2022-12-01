import { InfoCircleFilled } from '@ant-design/icons';
import { Card, Form, Input, Space, Typography } from 'antd';
import React from 'react';

function EditSAML(): JSX.Element {
	return (
		<>
			<Form.Item
				label="SAML ACS URL"
				name={['ssoConfig', 'samlIdp']}
				rules={[{ required: true, message: 'Please input your ACS URL!' }]}
			>
				<Input />
			</Form.Item>

			<Form.Item
				label="SAML Entity ID"
				name={['ssoConfig', 'samlEntity']}
				rules={[{ required: true, message: 'Please input your Entity Id!' }]}
			>
				<Input />
			</Form.Item>

			<Form.Item
				rules={[{ required: true, message: 'Please input your Certificate!' }]}
				label="SAML X.509 Certificate"
				name={['ssoConfig', 'samlCert']}
			>
				<Input.TextArea rows={4} />
			</Form.Item>

			<Card style={{ marginBottom: '1rem' }}>
				<Space>
					<InfoCircleFilled />
					<Typography>
						SAML won’t be enabled unless you enter all the attributes above
					</Typography>
				</Space>
			</Card>
		</>
	);
}

export default EditSAML;
