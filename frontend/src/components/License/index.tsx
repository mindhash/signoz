import { Button, Form, Input, Modal, Typography } from 'antd';
import applyLicenseAPI from 'api/license/apply';
import React, { useState } from 'react';

interface LicenseModalProps {
	visible: boolean;
	setVisible: (b: boolean) => void;
	title?: string;
}

function LicenseModal({
	setVisible,
	visible,
	title,
}: LicenseModalProps): JSX.Element {
	const [key, setKey] = useState('');
	const [applying, setApplying] = useState<boolean>(false);
	const [feedback, setFeedback] = useState('');
	const [applied, setApplied] = useState(false);
	const applyLicense = async (): Promise<void> => {
		setApplying(true);
		try {
			if (key === '') {
				setFeedback('please enter a valid key!');
			}
			const response = await applyLicenseAPI({ key });
			if (response.statusCode !== 200) {
				setFeedback('failed to apply license key, please contact signoz support');
			} else {
				setFeedback(
					`License key (${key}) applied successfully. Please refresh the page to see updates.`,
				);
				setApplied(true);
			}
		} catch (e) {
			console.log('failed to apply license key', e);
			setFeedback('something went wrong, please contact your admin');
		}
		setApplying(false);
	};

	const renderFeedback = (): JSX.Element => {
		return (
			<Typography.Paragraph style={{ color: '#f14' }}>
				{feedback}
			</Typography.Paragraph>
		);
	};
	const renderForm = (): JSX.Element => {
		return (
			<Form>
				<Form.Item
					label="License Key"
					name="licenseKey"
					rules={[
						{
							required: true,
							message: (
								<p>
									Please enter a license key! If you dont have one,{' '}
									<a href="https://signoz.io" target="_blank" rel="noreferrer">
										contact sales
									</a>{' '}
								</p>
							),
						},
					]}
				>
					<Input
						onChange={(e): void => {
							setKey(e.target.value);
						}}
					/>
				</Form.Item>
				<Typography.Paragraph>
					Don&#39;t have a license yet?
					<div>
						<Typography.Link>
							<a href="https://signoz.io" target="_blank" rel="noreferrer">
								See Plans
							</a>
						</Typography.Link>
					</div>
				</Typography.Paragraph>
				{feedback && renderFeedback()}
			</Form>
		);
	};
	return (
		<Modal
			title={title}
			visible={visible}
			onCancel={(): void => setVisible(false)}
			closable={!applied}
			centered
			destroyOnClose
			footer={[
				<Button
					key="back"
					disabled={applied}
					onClick={(): void => setVisible(false)}
					type="default"
				>
					Cancel
				</Button>,
				<Button
					key="applyLicense"
					onClick={(e): void => {
						e.preventDefault();
						applyLicense();
					}}
					type="primary"
					disabled={applied}
					loading={applying}
				>
					Apply
				</Button>,
			]}
		>
			{!applied && renderForm()}
			{applied && renderFeedback()}
		</Modal>
	);
}

LicenseModal.defaultProps = {
	title: 'Upgrade Your Plan',
};

export default LicenseModal;
