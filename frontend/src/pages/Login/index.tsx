import { Typography } from 'antd';
import getUserVersion from 'api/user/getVersion';
import Spinner from 'components/Spinner';
import WelcomeLeftContainer from 'components/WelcomeLeftContainer';
import LoginContainer from 'container/Login';
import React from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from 'react-query';
import { useSelector } from 'react-redux';
import { useLocation } from 'react-router-dom';
import { AppState } from 'store/reducers';
import AppReducer from 'types/reducer/app';

function useURLQuery(): URLSearchParams {
	const { search } = useLocation();

	return React.useMemo(() => new URLSearchParams(search), [search]);
}

function Login(): JSX.Element {
	const { isLoggedIn } = useSelector<AppState, AppReducer>((state) => state.app);
	const { t } = useTranslation();
	const urlQueryParams = useURLQuery();
	const jwt = urlQueryParams.get('jwt') || '';
	const refreshJwt = urlQueryParams.get('refreshJwt') || '';
	const userId = urlQueryParams.get('usr') || '';

	const versionResult = useQuery({
		queryFn: getUserVersion,
		queryKey: 'getUserVersion',
		enabled: !isLoggedIn,
	});

	if (versionResult.status === 'error') {
		return (
			<Typography>
				{versionResult.data?.error || t('something_went_wrong')}
			</Typography>
		);
	}

	if (
		versionResult.status === 'loading' ||
		!(versionResult.data && versionResult.data.payload)
	) {
		return <Spinner tip="Loading..." />;
	}

	const { version } = versionResult.data.payload;

	return (
		<WelcomeLeftContainer version={version}>
			<LoginContainer jwt={jwt} refreshJwt={refreshJwt} userId={userId} />
		</WelcomeLeftContainer>
	);
}

export default Login;
