import React from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import {
  AppBar,
  Toolbar,
  Typography,
  Button,
  Box,
  Container,
  useTheme,
} from '@mui/material';
import HomeIcon from '@mui/icons-material/Home';
import ListAltIcon from '@mui/icons-material/ListAlt';

const NavBar = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const theme = useTheme();

  const isActive = (path) => {
    return location.pathname === path;
  };

  return (
    <AppBar position="static" color="transparent">
      <Container maxWidth="xl">
        <Toolbar>
          <Typography
            variant="h6"
            component="div"
            sx={{
              flexGrow: 1,
              display: 'flex',
              alignItems: 'center',
              color: theme.palette.primary.main,
              fontWeight: 'bold',
            }}
          >
            <img 
              src="/logo192.png" 
              alt="Logo" 
              style={{ height: '32px', marginRight: '10px' }} 
            />
            AutoLogin Manager
          </Typography>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button
              color={isActive('/') ? 'primary' : 'inherit'}
              onClick={() => navigate('/')}
              startIcon={<HomeIcon />}
              variant={isActive('/') ? 'contained' : 'text'}
            >
              Home
            </Button>
            <Button
              color={isActive('/jobs') ? 'primary' : 'inherit'}
              onClick={() => navigate('/jobs')}
              startIcon={<ListAltIcon />}
              variant={isActive('/jobs') ? 'contained' : 'text'}
            >
              Jobs
            </Button>
          </Box>
        </Toolbar>
      </Container>
    </AppBar>
  );
};

export default NavBar; 