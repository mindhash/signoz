import { Button, Input, notification, Space, Typography } from 'antd';
import loginApi from 'api/user/login';
import loginPrecheckApi from 'api/user/loginPrecheck';
import afterLogin from 'AppRoutes/utils';
import ROUTES from 'constants/routes';
import history from 'lib/history';
import React, { useEffect, useState } from 'react';

import { FormContainer, FormWrapper, Label, ParentContainer } from './styles';

const { Title } = Typography;

function Login({
	jwt,
	refreshJwt,
	userId,
}: {
	jwt: string;
	refreshJwt: string;
	userId: string;
}): JSX.Element {
	const [isLoading, setIsLoading] = useState<boolean>(false);
	const [email, setEmail] = useState<string>('');
	const [password, setPassword] = useState<string>('');
	const [enforceSAML, setEnforceSAML] = useState<boolean>(false);
	const [samlLoginUrl, setSamlLoginUrl] = useState<string>('');
	const [precheckError, setPrecheckError] = useState<string>('');
	const [precheckComplete, setPrecheckComplete] = useState<boolean>(false);
	useEffect(() => {
		async function processJwt(): Promise<void> {
			if (jwt && jwt !== '') {
				setIsLoading(true);
				await afterLogin(userId, jwt, refreshJwt);
				setIsLoading(false);
				history.push(ROUTES.APPLICATION);
			}
		}
		processJwt();
	}, [jwt, refreshJwt, userId]);

	const onBlurHandler = async (value: string): Promise<void> => {
		const response = await loginPrecheckApi({
			email: value,
		});
		setPrecheckComplete(true);
		if (response.statusCode === 200) {
			const { samlEnabled, samlLoginUrl } = response.payload;
			if (samlEnabled) {
				setEnforceSAML(true);
				if (samlLoginUrl !== '') {
					setSamlLoginUrl(samlLoginUrl);
				} else {
					setPrecheckError('Invalid SAML setup, please contact your administrator');
				}
			}
		} else {
			setPrecheckError('Invalid auth setup, please contact your administrator');
		}
	};
	const onChangeHandler = (
		setFunc: React.Dispatch<React.SetStateAction<string>>,
		value: string,
	): void => {
		setFunc(value);
	};

	const onSubmitHandler: React.FormEventHandler<HTMLFormElement> = async (
		event,
	) => {
		try {
			event.preventDefault();
			event.persist();
			setIsLoading(true);

			const response = await loginApi({
				email,
				password,
			});
			if (response.statusCode === 200) {
				await afterLogin(
					response.payload.userId,
					response.payload.accessJwt,
					response.payload.refreshJwt,
				);
				history.push(ROUTES.APPLICATION);
			} else {
				notification.error({
					message: response.error || 'Something went wrong',
				});
			}
			setIsLoading(false);
		} catch (error) {
			setIsLoading(false);
			notification.error({
				message: 'Something went wrong',
			});
		}
	};

	const renderSAMLAction = (): JSX.Element => {
		return <a href={samlLoginUrl}>Login with SSO</a>;
	};

	return (
		<FormWrapper>
			<FormContainer onSubmit={onSubmitHandler}>
				<Title level={4}>Login to SigNoz</Title>
				<ParentContainer>
					<Label htmlFor="signupEmail">Email</Label>
					<Input
						placeholder="name@yourcompany.com"
						type="email"
						autoFocus
						required
						id="loginEmail"
						onChange={(event): void => onChangeHandler(setEmail, event.target.value)}
						onBlur={(event): Promise<void> => onBlurHandler(event.target.value)}
						value={email}
						disabled={isLoading}
					/>
				</ParentContainer>
				{precheckComplete && !enforceSAML && (
					<ParentContainer>
						<Label htmlFor="Password">Password</Label>
						<Input.Password
							required
							id="currentPassword"
							onChange={(event): void =>
								onChangeHandler(setPassword, event.target.value)
							}
							disabled={isLoading}
							value={password}
						/>
					</ParentContainer>
				)}
				<Space
					style={{ marginTop: '1.3125rem' }}
					align="start"
					direction="vertical"
					size={20}
				>
					{!precheckComplete && (
						<Button disabled={isLoading} loading={isLoading} type="primary">
							Next
						</Button>
					)}
					{precheckComplete && !enforceSAML && (
						<Button
							disabled={isLoading}
							loading={isLoading}
							type="primary"
							htmlType="submit"
							data-attr="signup"
						>
							Login
						</Button>
					)}
					{precheckComplete && enforceSAML && renderSAMLAction()}
					<Typography.Paragraph style={{ color: '#f14' }}>
						{precheckError}
					</Typography.Paragraph>
					<Typography.Link
						onClick={(): void => {
							history.push(ROUTES.SIGN_UP);
						}}
						style={{ fontWeight: 700 }}
					>
						Create an account
					</Typography.Link>

					<Typography.Paragraph italic style={{ color: '#ACACAC' }}>
						If you have forgotten you password, ask your admin to reset password and
						send you a new invite link
					</Typography.Paragraph>
				</Space>
			</FormContainer>
		</FormWrapper>
	);
}

export default Login;
