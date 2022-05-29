import React, { useState } from 'react';
import { Modal, Input, Button, Form, Typography } from 'antd';

interface UpgradeModalProps {
	visible: boolean;
	setVisible: (b: boolean) => void;
	title?: string;
}

function UpgradeModal({
	setVisible,
	visible,
	title = 'Upgrade Your Plan',
}: UpgradeModalProps): JSX.Element {
	const [showApply, setShowApply] = useState<boolean>(false);
	const [applying, setApplying] = useState<boolean>(false);
	return (
		<Modal
			title={title}
			visible={visible}
			onCancel={(): void => setVisible(false)}
			centered
			destroyOnClose
			footer={[
				<Button key="back" onClick={(): void => setVisible(false)} type="default">
					Cancel
				</Button>,
				<Button
					key="upgrade_modal"
					onClick={(e): void => {
						e.preventDefault();
					}}
					type="primary"
					disabled={showApply}
					loading={applying}
				>
					Apply
				</Button>,
			]}
		>
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
									<a href="/">contact sales</a>{' '}
								</p>
							),
						},
					]}
				>
					<Input />
				</Form.Item>
				<Typography.Paragraph>
					Don&#39;t have a license yet?
					<div>
						<Typography.Link>
							<a href="https://signoz.io" target="_blank">
								See Plans
							</a>
						</Typography.Link>
					</div>
				</Typography.Paragraph>
			</Form>
		</Modal>
	);
}

export default UpgradeModal;
